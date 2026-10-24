package utils

import (
	"aura/config"
	"aura/models"
	"fmt"
	maps0 "maps"
	"regexp"
	"strings"
	"time"
)

var templateVarRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// RenderTemplate replaces {{Variable}} tokens with values from vars.
// Unknown variables are left unchanged.
func RenderTemplate(input string, vars map[string]string) string {
	if input == "" {
		return input
	}

	return templateVarRegex.ReplaceAllStringFunc(input, func(token string) string {
		m := templateVarRegex.FindStringSubmatch(token)
		if len(m) < 2 {
			return token
		}
		key := strings.TrimSpace(m[1])
		if val, ok := vars[key]; ok {
			return val
		}
		return token
	})
}

func MergeTemplateVars(maps ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, m := range maps {
		maps0.Copy(out, m)
	}
	return out
}

func BaseTemplateVars() map[string]string {
	return map[string]string{
		"AppName":         config.AppName,
		"AppVersion":      config.AppVersion,
		"AppPort":         fmt.Sprintf("%d", config.AppPort),
		"AppAuthor":       config.AppAuthor,
		"AppLicense":      config.AppLicense,
		"MediaServerName": config.MediaServerName,
		"MediaServerType": config.Current.MediaServer.Type,
		"Timestamp":       time.Now().Format("2006-01-02 15:04:05"),
		"NewLine":         "\n",
		"Tab":             "\t",
	}
}

func TemplateVars_AppStartup(appName, appVersion string, appPort int) map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
	)
}

func TemplateVars_TestNotification() map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
	)
}

func TemplateVars_Autodownload(mediaItem models.MediaItem, setItem models.DBPosterSetDetail, image models.ImageFile, reasonTitle string, reason string) map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
		map[string]string{
			"MediaItemTitle":        mediaItem.Title,
			"MediaItemYear":         fmt.Sprintf("%d", mediaItem.Year),
			"MediaItemTMDBID":       mediaItem.TMDB_ID,
			"MediaItemLibraryTitle": mediaItem.LibraryTitle,
			"MediaItemRatingKey":    mediaItem.RatingKey,
			"MediaItemType":         mediaItem.Type,
			"SetID":                 setItem.ID,
			"SetTitle":              setItem.Title,
			"SetType":               setItem.Type,
			"SetCreator":            setItem.UserCreated,
			"ImageName":             GetFileDownloadName(mediaItem.Title, image),
			"ImageType":             image.Type,
			"ReasonTitle":           reasonTitle,
			"Reason":                reason,
		},
	)
}

func TemplateVars_DownloadQueue(mediaItem models.MediaItem, setItem models.DBPosterSetDetail, Errors []string, Warnings []string) map[string]string {
	var result string
	if len(Errors) > 0 {
		result = "Error"
	} else if len(Warnings) > 0 {
		result = "Warning"
	} else {
		result = "Success"
	}
	reasonTitle := result
	var reason string
	switch result {
	case "Success":
		reason = "Download completed successfully with no issues detected."
	case "Warning":
		reason = fmt.Sprintf("Download completed with warnings:\n%s", strings.Join(Warnings, "\n"))
	case "Error":
		reason = fmt.Sprintf("Download completed with errors:\n%s", strings.Join(Errors, "\n"))
		if len(Warnings) > 0 {
			reason += fmt.Sprintf("\n\n and the following Warnings:\n%s", strings.Join(Warnings, "\n"))
		}
	}
	return MergeTemplateVars(
		BaseTemplateVars(),
		map[string]string{
			"MediaItemTitle":        mediaItem.Title,
			"MediaItemYear":         fmt.Sprintf("%d", mediaItem.Year),
			"MediaItemTMDBID":       mediaItem.TMDB_ID,
			"MediaItemLibraryTitle": mediaItem.LibraryTitle,
			"MediaItemRatingKey":    mediaItem.RatingKey,
			"MediaItemType":         mediaItem.Type,
			"SetID":                 setItem.ID,
			"SetTitle":              setItem.Title,
			"SetType":               setItem.Type,
			"SetCreator":            setItem.UserCreated,
			"ReasonTitle":           reasonTitle,
			"Reason":                reason,
		},
	)
}

func TemplateVars_NewSetsAvailableForIgnoredItems(mediaItem models.MediaItem, setCount int) map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
		map[string]string{
			"MediaItemTitle":        mediaItem.Title,
			"MediaItemYear":         fmt.Sprintf("%d", mediaItem.Year),
			"MediaItemTMDBID":       mediaItem.TMDB_ID,
			"MediaItemLibraryTitle": mediaItem.LibraryTitle,
			"MediaItemRatingKey":    mediaItem.RatingKey,
			"MediaItemType":         mediaItem.Type,
			"SetCount":              fmt.Sprintf("%d", setCount),
		},
	)
}

func TemplateVars_CheckForMediaItemChangesJob(mediaItem models.MediaItem, reason string, action string, moreInfo string) map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
		map[string]string{
			"MediaItemTitle":        mediaItem.Title,
			"MediaItemYear":         fmt.Sprintf("%d", mediaItem.Year),
			"MediaItemTMDBID":       mediaItem.TMDB_ID,
			"MediaItemLibraryTitle": mediaItem.LibraryTitle,
			"MediaItemRatingKey":    mediaItem.RatingKey,
			"MediaItemType":         mediaItem.Type,
			"Reason":                reason,
			"Action":                action,
			"MoreInfo":              moreInfo,
		},
	)
}

func TemplateVars_SonarrNotification(mediaItem models.MediaItem, setItem models.DBPosterSetDetail, image models.ImageFile, reasonTitle string, reason string, result string) map[string]string {
	return MergeTemplateVars(
		BaseTemplateVars(),
		map[string]string{
			"MediaItemTitle":        mediaItem.Title,
			"MediaItemYear":         fmt.Sprintf("%d", mediaItem.Year),
			"MediaItemTMDBID":       mediaItem.TMDB_ID,
			"MediaItemLibraryTitle": mediaItem.LibraryTitle,
			"MediaItemRatingKey":    mediaItem.RatingKey,
			"MediaItemType":         mediaItem.Type,
			"SetID":                 setItem.ID,
			"SetTitle":              setItem.Title,
			"SetType":               setItem.Type,
			"SetCreator":            setItem.UserCreated,
			"ImageName":             GetFileDownloadName(mediaItem.Title, image),
			"ImageType":             image.Type,
			"ReasonTitle":           reasonTitle,
			"Reason":                reason,
			"Result":                result,
		},
	)
}

// TemplateVars_NewItemAutoDownload is TemplateVars_DownloadQueue for a set downloaded automatically from a favorite creator
func TemplateVars_NewItemAutoDownload(mediaItem models.MediaItem, setItem models.DBPosterSetDetail, Errors []string, Warnings []string, newItem bool) map[string]string {
	vars := TemplateVars_DownloadQueue(mediaItem, setItem, Errors, Warnings)
	found := fmt.Sprintf("This item had no saved set and %s found a set by one of your favorite creators \"%s\".", config.AppName, setItem.UserCreated)
	if newItem {
		found = fmt.Sprintf("A new media item was detected and %s found a set by one of your favorite creators \"%s\".", config.AppName, setItem.UserCreated)
	}
	switch vars["ReasonTitle"] {
	case "Success":
		vars["Reason"] = found + " This set was automatically downloaded with no issues."
	case "Warning":
		vars["Reason"] = fmt.Sprintf("%s This set was automatically downloaded with warnings:\n%s", found, strings.Join(Warnings, "\n"))
	default:
		vars["Reason"] = fmt.Sprintf("%s This set was automatically downloaded with errors:\n%s", found, strings.Join(Errors, "\n"))
		if len(Warnings) > 0 {
			vars["Reason"] += fmt.Sprintf("\n\n and the following Warnings:\n%s", strings.Join(Warnings, "\n"))
		}
	}
	return vars
}

// TemplateVars_SiloPlugin is TemplateVars_DownloadQueue for a set Silo used from the aura Silo plugin
func TemplateVars_SiloPlugin(mediaItem models.MediaItem, setItem models.DBPosterSetDetail, Errors []string, Warnings []string) map[string]string {
	vars := TemplateVars_DownloadQueue(mediaItem, setItem, Errors, Warnings)
	used := fmt.Sprintf("Silo used a set by \"%s\" from the %s Silo Plugin, so %s downloaded the whole set", setItem.UserCreated, config.AppName, config.AppName)
	switch vars["ReasonTitle"] {
	case "Success":
		vars["Reason"] = used + " with no issues."
	case "Warning":
		vars["Reason"] = fmt.Sprintf("%s with warnings:\n%s", used, strings.Join(Warnings, "\n"))
	default:
		vars["Reason"] = fmt.Sprintf("%s with errors:\n%s", used, strings.Join(Errors, "\n"))
		if len(Warnings) > 0 {
			vars["Reason"] += fmt.Sprintf("\n\n and the following Warnings:\n%s", strings.Join(Warnings, "\n"))
		}
	}
	return vars
}
