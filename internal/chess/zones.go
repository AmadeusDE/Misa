package chess

import "fmt"

type moveZoneDef struct {
	Zone  []string `yaml:"zone"`
	Betza string   `yaml:"betza"`
}

type moveZone struct {
	zone  []bool
	moves []Component
}

func (v *Variant) squareZone(names []string) ([]bool, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("zone must not be empty")
	}
	zone := make([]bool, v.Size())
	for _, name := range names {
		sq, err := v.ParseSquare(name)
		if err != nil {
			return nil, err
		}
		if zone[sq] {
			return nil, fmt.Errorf("duplicate zone square %s", name)
		}
		zone[sq] = true
	}
	return zone, nil
}

func (v *Variant) buildZones(t *PieceType, d pieceDef) error {
	var err error
	if d.Zone != nil {
		t.zone, err = v.squareZone(d.Zone)
		if err != nil {
			return err
		}
	}
	occupied := make([]bool, v.Size())
	for _, def := range d.MoveZones {
		zone, err := v.squareZone(def.Zone)
		if err != nil {
			return fmt.Errorf("move_zones: %v", err)
		}
		for sq, present := range zone {
			if present {
				if occupied[sq] {
					return fmt.Errorf("move_zones overlap at %s", v.SquareName(sq))
				}
				occupied[sq] = true
			}
		}
		moves, err := ParseBetza(def.Betza)
		if err != nil {
			return fmt.Errorf("move_zones: %v", err)
		}
		t.moveZones = append(t.moveZones, moveZone{zone, moves})
	}
	if d.RoyalAttack != nil {
		t.royalAttack, err = ParseBetza(*d.RoyalAttack)
		if err != nil {
			return fmt.Errorf("royal_attack: %v", err)
		}
	}
	return nil
}

func (t *PieceType) allows(v *Variant, sq int, c Color) bool {
	return t.zone == nil || t.zone[v.orient(sq, c)]
}

func (t *PieceType) movesAt(v *Variant, sq int, c Color) []Component {
	for _, z := range t.moveZones {
		if z.zone[v.orient(sq, c)] {
			return z.moves
		}
	}
	return t.moves
}
