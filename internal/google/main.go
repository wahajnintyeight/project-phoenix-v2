package google

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Google interface {
	SearchYoutube(searchQuery string)
}

var (
	logger = log.New(os.Stdout, "GOOGLE: ", log.LstdFlags)
)

func GetAPIKey() string {
	godotenv.Load()
	API_KEY := os.Getenv("GOOGLE_API_KEY")
	return API_KEY
}

func SearchYoutube(searchQuery string, maxResults int64, nextPage string, prevPage string) (interface{}, error){

	ctx := context.Background()
	logger.Println("Creating YouTube service")
	service, err := youtube.NewService(ctx, option.WithAPIKey(GetAPIKey()))
	if err != nil {
		log.Fatalf("Error creating YouTube service: %v", err)
	}
	api := service.Search.List([]string{"snippet"}).Q(searchQuery).MaxResults(maxResults).Type("video").Order("relevance")

	if nextPage != "" {
		api.PageToken(nextPage)
	}
	res, e := api.Do()
	if e != nil {
		return nil, e
	} else {
		return map[string]interface{}{
			"items": res.Items,
			"totalPage": res.PageInfo.TotalResults,
			"nextPage": res.NextPageToken,
			"prevPage": res.PrevPageToken,
		}, nil
	}
 
}