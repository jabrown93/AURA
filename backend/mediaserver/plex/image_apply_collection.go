package plex

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"net/url"
	"path"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (p *Plex) ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Applying %s Image to Collection '%s' | %s [%s]",
		cases.Title(language.English).String(imageFile.Type), collectionItem.Title, collectionItem.LibraryTitle, collectionItem.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Get the MediUX Image URL
	imageURL, Err := mediux.ConstructImageUrl(ctx, imageFile.ID, imageFile.Modified.String(), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return *logAction.Error
	}

	// POST Method is used when not using a local image
	// POST Method requires the posterType to be plural (posters or arts)
	var imageType string
	if imageFile.Type == "collection_backdrop" {
		imageType = "arts"
	} else {
		imageType = "posters"
	}

	// Construct the URL for the Plex API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", collectionItem.RatingKey, imageType)
	query := u.Query()
	query.Set("url", imageURL)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, _, Err := makeRequest(ctx, config.Current.MediaServer, URL, "POST", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	// If Kometa mode is enabled, also write the collection image into the Kometa asset
	// directory. This is non-fatal: a failure to write the asset must not fail the apply.
	if config.Current.Images.Kometa.Enabled {
		imageData, _, imgErr := mediux.GetImage(ctx, imageFile.ID, imageFile.Modified.Format("20060102150405"), mediux.ImageQualityOriginal)
		if imgErr.Message != "" {
			logAction.AppendWarning("kometa_save_failed", map[string]any{"error": imgErr.Message})
		} else if kometaErr := SaveCollectionImageKometa(ctx, collectionItem, imageFile, imageData); kometaErr.Message != "" {
			logAction.AppendWarning("kometa_save_failed", map[string]any{"error": kometaErr.Message})
		}
	}

	return Err
}
