package plex

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
)

func (p *Plex) AddLabelToMediaItem(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Plex: Adding Labels to %s", utils.MediaItemInfo(item)),
		logging.LevelInfo)
	defer logAction.Complete()

	// Do one last check to ensure we are a Plex server
	if p.Config.Type != "Plex" {
		return logging.LogErrorInfo{}
	} else if len(config.Current.LabelsAndTags.Applications) == 0 {
		return logging.LogErrorInfo{}
	}

	if item.Type != "movie" && item.Type != "show" {
		logAction.AppendWarning("outcome", "skipped")
		logAction.AppendWarning("reason", "unsupported_media_type")
		return logging.LogErrorInfo{}
	} else if item.RatingKey == "" {
		logAction.AppendWarning("outcome", "skipped")
		logAction.AppendWarning("reason", "missing_rating_key")
		return logging.LogErrorInfo{}
	} else if item.LibraryTitle == "" {
		logAction.AppendWarning("outcome", "skipped")
		logAction.AppendWarning("reason", "missing_library_title")
		return logging.LogErrorInfo{}
	} else if item.TMDB_ID == "" {
		logAction.AppendWarning("outcome", "skipped")
		logAction.AppendWarning("reason", "missing_tmdb_id")
		return logging.LogErrorInfo{}
	}

	// Get all of the applications configured for labels and tags
	for _, app := range config.Current.LabelsAndTags.Applications {
		if app.Application != "Plex" {
			continue
		}

		ctx, subAppAction := logging.AddSubActionToContext(ctx, "Processing Plex Labels", logging.LevelDebug)
		defer subAppAction.Complete()

		// Check if we are enabled for this application
		if !app.Enabled {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "application disabled")
			continue
		}

		// Check to see there at least one label to add or remove
		if len(app.Add) == 0 && len(app.Remove) == 0 {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "no labels to add or remove")
			continue
		}

		// Get the library section from the cache
		librarySection, found := cache.MediaServerLibraryStore(p.Config.ID, p.Config.IsMain()).GetSectionByTitle(item.LibraryTitle)
		if !found {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "library section not found in cache")
			subAppAction.AppendWarning("library_title", item.LibraryTitle)
			continue
		} else if librarySection.ID == "" {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "library section missing id in cache")
			subAppAction.AppendWarning("library_title", item.LibraryTitle)
			continue
		}

		typeNumber := 1
		if item.Type == "show" {
			typeNumber = 2
		}
		subAppAction.AppendResult("type_number", typeNumber)

		// Make a comma-separated string of labels to remove
		labelsToRemove := ""
		if len(app.Remove) > 0 {
			if config.Current.LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload && !selectedTypes.Poster {
				// If the "RemoveOverlayLabelOnlyOnPosterDownload" setting is enabled and the selected types do not include Poster, filter out "Overlay" from the removal list
				filteredLabels := []string{}
				for _, label := range app.Remove {
					if label != "Overlay" {
						filteredLabels = append(filteredLabels, label)
					}
				}
				labelsToRemove = strings.Join(filteredLabels, ",")
			} else {
				labelsToRemove = strings.Join(app.Remove, ",")
			}
		}

		// %5B = [
		// %5D = ]
		// Construct the removal parameter for the URL
		// Structure: label%5B%5D.tag.tag-={label1},{label2}
		// Example: label%5B%5D.tag.tag-=Overlay,4K
		removalParam := ""
		if labelsToRemove != "" {
			removalParam = fmt.Sprintf("label%%5B%%5D.tag.tag-=%s", url.QueryEscape(labelsToRemove))
			subAppAction.AppendResult("labels_to_remove", app.Remove)
		}

		// Construct the addition parameter for the URL
		// Structure: label%5B{index}%5D.tag.tag={label1},{label2}
		// Example: label%5B0%5D.tag.tag=Overlay&label%5B0%5D.tag.tag=4K
		// Note: The index should start at 0 and increment for each label to add
		labelsToAdd := ""
		additionParams := ""
		if len(app.Add) > 0 || (app.AddLabelTagForSelectedTypes && (selectedTypes.Poster || selectedTypes.Backdrop || selectedTypes.SeasonPoster || selectedTypes.SpecialSeasonPoster || selectedTypes.Titlecard)) {
			nextLabelIndex := len(app.Add)
			for index, label := range app.Add {
				if index > 0 {
					additionParams += "&"
				}
				additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", index, url.QueryEscape(label))
				labelsToAdd += label
				if index < len(app.Add)-1 {
					labelsToAdd += ","
				}
			}
			if app.AddLabelTagForSelectedTypes && (selectedTypes.Poster || selectedTypes.Backdrop || selectedTypes.SeasonPoster || selectedTypes.SpecialSeasonPoster || selectedTypes.Titlecard) {
				if selectedTypes.Poster {
					if labelsToAdd != "" {
						labelsToAdd += ","
					}
					labelsToAdd += "aura-poster"
					if additionParams != "" {
						additionParams += "&"
					}
					additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", nextLabelIndex, url.QueryEscape("aura-poster"))
					nextLabelIndex++
				}
				if selectedTypes.Backdrop {
					if labelsToAdd != "" {
						labelsToAdd += ","
					}
					labelsToAdd += "aura-backdrop"
					if additionParams != "" {
						additionParams += "&"
					}
					additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", nextLabelIndex, url.QueryEscape("aura-backdrop"))
					nextLabelIndex++
				}
				if selectedTypes.SeasonPoster {
					if labelsToAdd != "" {
						labelsToAdd += ","
					}
					labelsToAdd += "aura-season-poster"
					if additionParams != "" {
						additionParams += "&"
					}
					additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", nextLabelIndex, url.QueryEscape("aura-season-poster"))
					nextLabelIndex++
				}
				if selectedTypes.SpecialSeasonPoster {
					if labelsToAdd != "" {
						labelsToAdd += ","
					}
					labelsToAdd += "aura-special-season-poster"
					if additionParams != "" {
						additionParams += "&"
					}
					additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", nextLabelIndex, url.QueryEscape("aura-special-season-poster"))
					nextLabelIndex++
				}
				if selectedTypes.Titlecard {
					if labelsToAdd != "" {
						labelsToAdd += ","
					}
					labelsToAdd += "aura-titlecard"
					if additionParams != "" {
						additionParams += "&"
					}
					additionParams += fmt.Sprintf("label%%5B%d%%5D.tag.tag=%s", nextLabelIndex, url.QueryEscape("aura-titlecard"))
					nextLabelIndex++
				}
			}
		}
		if labelsToAdd != "" {
			subAppAction.AppendResult("labels_to_add", strings.Split(labelsToAdd, ","))
		}

		// If no labels to add or remove, return early
		if removalParam == "" && additionParams == "" {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "no labels to add or remove after processing")
			continue
		}

		// Combine removal and addition parameters
		var combinedParams string
		if labelsToRemove != "" && additionParams != "" {
			combinedParams = fmt.Sprintf("%s&%s", removalParam, additionParams)
		} else if labelsToRemove != "" {
			combinedParams = fmt.Sprintf("label%%5B%%5D.tag.tag-=%s", url.QueryEscape(labelsToRemove))
		} else if additionParams != "" {
			combinedParams = additionParams
		} else {
			subAppAction.AppendWarning("outcome", "skipped")
			subAppAction.AppendWarning("reason", "no labels to add or remove after processing")
			continue
		}

		// Construct the URL for the Plex API request
		u, err := url.Parse(p.Config.URL)
		if err != nil {
			subAppAction.SetError("failed to parse Plex URL",
				"Ensure that the Plex URL in the configuration is valid",
				map[string]any{
					"error": err.Error(),
					"url":   p.Config.URL,
				})
			continue
		}
		u.Path = path.Join(u.Path, "library", "sections", librarySection.ID, "all")
		query := u.Query()
		query.Set("type", fmt.Sprintf("%d", typeNumber))
		query.Set("id", item.RatingKey)
		u.RawQuery = query.Encode()
		URL := u.String()
		URL += "&" + combinedParams

		// Make the API request to add/remove labels
		_, _, Err := makeRequest(ctx, p.Config, URL, "PUT", nil)
		if Err.Message != "" {
			continue
		}

		subAppAction.AppendResult("outcome", "success")
	}

	return logging.LogErrorInfo{}
}
