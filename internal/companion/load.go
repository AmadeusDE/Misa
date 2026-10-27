package companion

import (
	"embed"
	"io/fs"
	"path"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed data
var dataFS embed.FS

type loadedData struct {
	catalog    catalogFile
	moods      []moodFile
	modifiers  []modifierFile
	yourKemono []kemonoFile
	compKemono []kemonoFile
}

var (
	dataOnce sync.Once
	dataAll  loadedData
	dataErr  error
)

func loadData() loadedData {
	dataOnce.Do(func() {
		dataAll, dataErr = readData()
	})
	return dataAll
}

func readData() (loadedData, error) {
	var out loadedData
	raw, err := dataFS.ReadFile("data/catalog.yaml")
	if err != nil {
		return out, err
	}
	if err := yaml.Unmarshal(raw, &out.catalog); err != nil {
		return out, err
	}
	err = fs.WalkDir(dataFS, "data", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		if path.Base(p) == "catalog.yaml" {
			return nil
		}
		b, err := dataFS.ReadFile(p)
		if err != nil {
			return err
		}
		dir := path.Dir(p)
		switch {
		case strings.Contains(dir, "/moods"):
			var m moodFile
			if err := yaml.Unmarshal(b, &m); err != nil {
				return err
			}
			if m.ID == "" {
				m.ID = strings.TrimSuffix(path.Base(p), ".yaml")
			}
			out.moods = append(out.moods, m)
		case strings.Contains(dir, "/modifiers"):
			var m modifierFile
			if err := yaml.Unmarshal(b, &m); err != nil {
				return err
			}
			if m.ID == "" {
				m.ID = strings.TrimSuffix(path.Base(p), ".yaml")
			}
			out.modifiers = append(out.modifiers, m)
		case strings.Contains(dir, "/kemonos/your"):
			var k kemonoFile
			if err := yaml.Unmarshal(b, &k); err != nil {
				return err
			}
			k.Kind = "your"
			if k.ID == "" {
				k.ID = strings.TrimSuffix(path.Base(p), ".yaml")
			}
			out.yourKemono = append(out.yourKemono, k)
		case strings.Contains(dir, "/kemonos/companion"):
			var k kemonoFile
			if err := yaml.Unmarshal(b, &k); err != nil {
				return err
			}
			k.Kind = "companion"
			if k.ID == "" {
				k.ID = strings.TrimSuffix(path.Base(p), ".yaml")
			}
			out.compKemono = append(out.compKemono, k)
		}
		return nil
	})
	return out, err
}

func HasThirsty(moods []string) bool {
	for _, m := range moods {
		if m == "thirsty" {
			return true
		}
	}
	return false
}

func normalizeMood(id string) string {
	switch id {
	case "chill", "":
		return "normal"
	case "yikes":
		return "thirsty"
	default:
		return id
	}
}
