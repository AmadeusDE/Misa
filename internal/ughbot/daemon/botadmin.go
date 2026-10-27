package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

var _ api.BotAdmin = (*Daemon)(nil)

// mergeStoredBots adds the bots made in the UI to cfg and returns their names.
// A stored bot whose name ughbot.toml also defines is an error rather than a silent winner.
func mergeStoredBots(cfg *config.Config, st *store.Store) (map[string]bool, error) {
	defs, err := st.BotDefs()
	if err != nil {
		return nil, err
	}
	ui := map[string]bool{}
	merged := *cfg
	merged.Bots = maps.Clone(cfg.Bots)
	if merged.Bots == nil {
		merged.Bots = map[string]config.Bot{}
	}
	for name, raw := range defs {
		def, err := config.DecodeStored([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("stored bot %q: %w", name, err)
		}
		def.Name = name
		if _, taken := cfg.Bots[name]; taken {
			return nil, fmt.Errorf("bot %q is defined in ughbot.toml and was also made in the UI; remove one", name)
		}
		merged.Bots[name] = def
		ui[name] = true
	}
	if err := merged.Validate(); err != nil {
		return nil, fmt.Errorf("stored bots: %w", err)
	}
	cfg.Bots = merged.Bots
	return ui, nil
}

func invalid(err error) error { return fmt.Errorf("%w: %v", api.ErrInvalid, err) }

func (d *Daemon) BotDef(_ context.Context, name string) (api.BotDef, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	def, ok := d.cfg.Bots[name]
	if !ok {
		return api.BotDef{}, fmt.Errorf("%w: %q", api.ErrUnknownBot, name)
	}
	return def, nil
}

func (d *Daemon) CreateBot(_ context.Context, def api.BotDef) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.cfg.Bots[def.Name]; ok {
		return fmt.Errorf("%w: %q", api.ErrBotExists, def.Name)
	}
	return d.install(def, nil)
}

func (d *Daemon) UpdateBot(_ context.Context, def api.BotDef) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	old, ok := d.bots[def.Name]
	if !ok {
		return fmt.Errorf("%w: %q", api.ErrUnknownBot, def.Name)
	}
	if !d.ui[def.Name] {
		return api.ErrReadOnly
	}
	return d.install(def, old)
}

func (d *Daemon) DeleteBot(_ context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	old, ok := d.bots[name]
	if !ok {
		return fmt.Errorf("%w: %q", api.ErrUnknownBot, name)
	}
	if !d.ui[name] {
		return api.ErrReadOnly
	}
	next, err := d.cfg.With(api.BotDef{Name: name}, true)
	if err != nil {
		return invalid(err)
	}
	if err := d.st.DeleteBotDef(name); err != nil {
		return err
	}
	if err := d.st.DeleteCodeGrants(name); err != nil {
		return err
	}
	old.Stop()
	delete(d.bots, name)
	delete(d.ui, name)
	d.cfg.Bots = next.Bots
	d.publishPeers()
	return nil
}

// install validates def, starts it in place of old (when set) and stores it. If the new
// bot cannot be built the old one is rebuilt from its previous definition.
func (d *Daemon) install(def api.BotDef, old *bot.Bot) error {
	next, err := d.cfg.With(def, false)
	if err != nil {
		return invalid(err)
	}
	def = next.Bots[def.Name]
	prev, hadPrev := d.cfg.Bots[def.Name]
	if old != nil {
		old.Stop()
	}
	defer d.publishPeers()
	nb, err := d.build(def)
	if err != nil {
		if hadPrev {
			if restored, rerr := d.build(prev); rerr == nil {
				d.bots[def.Name] = restored
			} else {
				delete(d.bots, def.Name)
				err = errors.Join(err, rerr)
			}
		}
		return invalid(err)
	}
	raw, _ := json.Marshal(def)
	if err := d.st.PutBotDef(def.Name, string(raw)); err != nil {
		nb.Stop()
		return err
	}
	d.cfg.Bots = next.Bots
	d.bots[def.Name] = nb
	d.ui[def.Name] = true
	return nil
}

func (d *Daemon) build(def config.Bot) (*bot.Bot, error) {
	b, err := d.newBot(def)
	if err != nil {
		return nil, err
	}
	if d.runCtx != nil {
		if err := b.Start(d.runCtx); err != nil {
			b.Stop()
			return nil, err
		}
	}
	return b, nil
}
