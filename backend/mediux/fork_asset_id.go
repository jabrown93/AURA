package mediux

// validAssetID reports whether id is a non-empty MediUX asset ID made of letters, digits, '-'
// and '_'. Fork: keeps a public asset_id from traversing out of the image cache folder.
func validAssetID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
