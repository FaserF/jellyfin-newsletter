package tmdb

import (
	"fmt"
	"strings"

	"github.com/SeaweedbrainCY/jellyfin-newsletter/internal/app"
	"github.com/SeaweedbrainCY/jellyfin-newsletter/internal/jellyfin"
	"go.uber.org/zap"
)

func enrichSeriesItemWithDefaultInfos(jellyfinSeriesItem *jellyfin.NewlyAddedSeriesItem) {
	defaultDetails := getDefaultItemDetails()
	jellyfinSeriesItem.Overview = defaultDetails.Overview
	jellyfinSeriesItem.PosterURL = defaultDetails.PosterURL
}

func applyJellyfinFallbackSeries(
	jellyfinSeriesItem *jellyfin.NewlyAddedSeriesItem,
	app *app.ApplicationContext,
) {
	baseURL := getJellyfinBaseURL(app)
	isPlaceholder := jellyfinSeriesItem.PosterURL == "" || jellyfinSeriesItem.PosterURL == DefaultPosterURL
	if isPlaceholder && baseURL != "" && jellyfinSeriesItem.SeriesID != "" {
		jellyfinSeriesItem.PosterURL = fmt.Sprintf(
			"%s/Items/%s/Images/Primary",
			strings.TrimRight(baseURL, "/"),
			jellyfinSeriesItem.SeriesID,
		)
	}
	if (jellyfinSeriesItem.Overview == "" || jellyfinSeriesItem.Overview == DefaultOverview) &&
		jellyfinSeriesItem.NativeOverview != "" {
		jellyfinSeriesItem.Overview = jellyfinSeriesItem.NativeOverview
	}
}

func EnrichSeriesItem(
	jellyfinSeriesItem *jellyfin.NewlyAddedSeriesItem,
	tmdbAPIClient APIInterface,
	app *app.ApplicationContext,
) {
	defer applyJellyfinFallbackSeries(jellyfinSeriesItem, app)

	if jellyfinSeriesItem.TMDBId != "" {
		parsedHTTPResponse, err := tmdbAPIClient.GetMediaByID(jellyfinSeriesItem.TMDBId, MediaTypeSeries)

		if err != nil {
			// Error is already logged by GetMediaByID
			enrichSeriesItemWithDefaultInfos(jellyfinSeriesItem)
			return
		}

		details := getItemDetailsFromHTTPResponse(parsedHTTPResponse)
		jellyfinSeriesItem.Overview = details.Overview
		jellyfinSeriesItem.PosterURL = details.PosterURL
		return
	}
	// No TMDB id, we perform a search by name and select the item with the highest popularity
	app.Logger.Debug(
		"Series has no TMDB id. TMDB information will be retrieved by searching with Series's name. If several media match, the choice will be based on the highest popularity.",
		zap.String("Series Name", jellyfinSeriesItem.SeriesName),
		zap.String("Series ID", jellyfinSeriesItem.SeriesID),
	)

	searchResult, err := tmdbAPIClient.SearchMediaByName(
		jellyfinSeriesItem.SeriesName,
		jellyfinSeriesItem.ProductionYear,
		MediaTypeSeries,
	)

	if err != nil {
		// Error is already logged by SearchMediaByName
		enrichSeriesItemWithDefaultInfos(jellyfinSeriesItem)
		return
	}

	details := getItemDetailsFromSearchResult(searchResult)

	jellyfinSeriesItem.Overview = details.Overview
	jellyfinSeriesItem.PosterURL = details.PosterURL
}

func EnrichSeriesItemsList(
	jellyfinSeriesItem *[]jellyfin.NewlyAddedSeriesItem,
	tmdbAPIClient APIInterface,
	app *app.ApplicationContext,
) {
	for i := range *jellyfinSeriesItem {
		EnrichSeriesItem(&(*jellyfinSeriesItem)[i], tmdbAPIClient, app)
	}
}
