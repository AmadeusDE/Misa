package lscore

type FilterFunc func(*FileInfo) bool

func FilterHidden(show bool) FilterFunc {
	if show {
		return nil
	}
	return func(fi *FileInfo) bool {
		return len(fi.Name) == 0 || fi.Name[0] != '.'
	}
}

func ApplyFilters(files []*FileInfo, filters ...FilterFunc) []*FileInfo {
	var active []FilterFunc
	for _, f := range filters {
		if f != nil {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return files
	}

	result := make([]*FileInfo, 0, len(files))
	for _, fi := range files {
		keep := true
		for _, f := range active {
			if !f(fi) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, fi)
		}
	}
	return result
}
