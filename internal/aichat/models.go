package aichat

import (
	"context"
	"sort"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/clipboard"
)

// providerModels lists "provider/model" for every provider that has a key or needs none, asking them in parallel.
// A provider that fails is reported in problems instead of hiding the rest.
func providerModels(ctx context.Context) (refs, problems []string) {
	infos, err := ai.Providers()
	if err != nil {
		return nil, []string{err.Error()}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, info := range infos {
		if info.KeySource == "" && !info.Local {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids, err := ai.Models(ctx, info.Name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				problems = append(problems, info.Name+": "+err.Error())
				return
			}
			for _, id := range ids {
				refs = append(refs, info.Name+"/"+id)
			}
		}()
	}
	wg.Wait()
	sort.Strings(refs)
	sort.Strings(problems)
	return refs, problems
}

// webTools are the search tools as ai.yaml configures them, nil when search is off.
func webTools() ([]ai.Tool, error) {
	s, err := ai.LoadSettings()
	if err != nil {
		return nil, err
	}
	return ai.WebTools(s.Search), nil
}

// copyText copies with the host tool, or asks the terminal to (OSC 52) when there is none.
func copyText(text string) tea.Cmd {
	if clipboard.Write(text) == nil {
		return nil
	}
	return tea.SetClipboard(text)
}
