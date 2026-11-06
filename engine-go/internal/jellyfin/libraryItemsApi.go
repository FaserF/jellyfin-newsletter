package jellyfin

import (
	"context"

	"github.com/SeaweedbrainCY/jellyfin-newsletter/internal/app"
	jellyfinAPI "github.com/sj14/jellyfin-go/api"
	"go.uber.org/zap"
)

const jellyfinIDLength = 32

type libraryItemAPI struct {
	jellyfinAPI.LibraryAPI

	structureAPI jellyfinAPI.LibraryStructureAPI
}

type Nullable[T any] interface {
	IsSet() bool
	Get() *T
}

func itemFieldsWithExclusions(
	baseFields []jellyfinAPI.ItemFields,
	app *app.ApplicationContext,
) []jellyfinAPI.ItemFields {
	if app != nil &&
		app.Config != nil &&
		(len(app.Config.Jellyfin.ExcludedPaths) > 0 || len(app.Config.Jellyfin.ExcludedGenres) > 0) {
		return append(baseFields, "Path", "Genres", "Tags", "Overview")
	}
	return baseFields
}

func (libraryAPI libraryItemAPI) GetItemsStats(app *app.ApplicationContext) (int32, int32, error) {
	itemsCounts, httpResponse, httpErr := libraryAPI.GetItemCounts(context.Background()).Execute()

	err := checkHTTPRequest("GetItemsStats", httpResponse, httpErr, app.Logger)
	if err != nil {
		return 0, 0, err
	}

	defer httpResponse.Body.Close()
	defer httpResponse.Body.Close()
	return *itemsCounts.MovieCount, *itemsCounts.EpisodeCount, nil
}

func (libraryAPI libraryItemAPI) GetMoviesItemsByFolderID(
	folderID string,
	app *app.ApplicationContext,
) (*[]jellyfinAPI.BaseItemDto, error) {
	fields := itemFieldsWithExclusions(
		[]jellyfinAPI.ItemFields{
			"DateCreated",
			"ProviderIds",
			"Id",
			"Name",
			"ProductionYear",
		},
		app,
	)
	movies, getMoviesHTTPResponse, httpErr := libraryAPI.GetItems(context.Background()).
		Recursive(false).
		EnableImages(false).
		ParentId(folderID).
		LocationTypes([]jellyfinAPI.LocationType{jellyfinAPI.LOCATIONTYPE_FILE_SYSTEM}).
		IsMovie(true).
		Fields(fields).
		Execute()

	err := checkHTTPRequest("GetMoviesItemsByFolderID", getMoviesHTTPResponse, httpErr, app.Logger)
	if err != nil {
		return nil, err
	}
	defer getMoviesHTTPResponse.Body.Close()
	app.Logger.Debug(
		"Successfully retrieved all movies from API",
		zap.String("Folder", folderID),
		zap.Any("Movies", movies.Items),
	)
	return &movies.Items, nil
}

func (libraryAPI libraryItemAPI) GetAllItemsByFolderID(
	folderID string,
	app *app.ApplicationContext,
) (*[]jellyfinAPI.BaseItemDto, error) {
	fields := itemFieldsWithExclusions(
		[]jellyfinAPI.ItemFields{
			"DateCreated",
			"ProviderIds",
			"Id",
			"Name",
			"ProductionYear",
			"IndexNumber",
			"SeriesId",
			"Type",
			"SeasonId",
		},
		app,
	)
	items, httpResponse, httpErr := libraryAPI.GetItems(context.Background()).
		Recursive(true).
		EnableImages(false).
		ParentId(folderID).
		Fields(fields).
		Execute()

	err := checkHTTPRequest("GetAllItemsByFolderID", httpResponse, httpErr, app.Logger)
	if err != nil {
		return nil, err
	}

	defer httpResponse.Body.Close()
	app.Logger.Debug(
		"Successfully retrieved all items from API",
		zap.String("Folder", folderID),
		zap.Any("Items", items.Items),
	)
	return &items.Items, nil
}

func findFileSystemFolderID(foldersItems *jellyfinAPI.BaseItemDtoQueryResult, folderName string) string {
	if !foldersItems.HasItems() {
		return ""
	}
	for _, item := range foldersItems.GetItems() {
		if (item.Name.IsSet() && item.Name.Get() != nil && *item.Name.Get() == folderName) ||
			(item.Id != nil && *item.Id == folderName) {
			return *item.Id
		}
	}
	return ""
}

func (libraryAPI libraryItemAPI) findVirtualFolderID(folderName string) string {
	if libraryAPI.structureAPI == nil {
		return ""
	}
	vFolders, vResp, vErr := libraryAPI.structureAPI.GetVirtualFolders(context.Background()).Execute()
	if vErr != nil || vResp == nil {
		return ""
	}
	defer vResp.Body.Close()
	for _, vf := range vFolders {
		if vf.GetName() == folderName || vf.GetItemId() == folderName {
			if vf.GetItemId() != "" {
				return vf.GetItemId()
			}
		}
	}
	return ""
}

func (libraryAPI libraryItemAPI) GetRootFolderIDByName(folderName string, app *app.ApplicationContext) (string, error) {
	foldersItems, httpResponse, httpErr := libraryAPI.GetItems(context.Background()).
		Recursive(false).
		LocationTypes([]jellyfinAPI.LocationType{jellyfinAPI.LOCATIONTYPE_FILE_SYSTEM}).
		Execute()

	err := checkHTTPRequest("GetRootFolderIDByName", httpResponse, httpErr, app.Logger)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()

	if id := findFileSystemFolderID(foldersItems, folderName); id != "" {
		return id, nil
	}
	if id := libraryAPI.findVirtualFolderID(folderName); id != "" {
		return id, nil
	}
	if len(folderName) == jellyfinIDLength {
		return folderName, nil
	}

	app.Logger.Warn("Folder not found. Will ignore it.", zap.String("folderName", folderName))
	return "", ErrItemsNotFound
}

func OrDefault[T any](n Nullable[T], def T) T {
	if n.IsSet() && n.Get() != nil {
		return *n.Get()
	}
	return def
}

func getTMDBIDIfExist(item *jellyfinAPI.BaseItemDto) string {
	if value, ok := item.ProviderIds["Tmdb"]; ok {
		if value == nil {
			return ""
		}
		return *value
	}
	return ""
}
