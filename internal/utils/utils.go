package utils

import (
	"github.com/momentohq/client-sdk-go/auth"
	"github.com/momentohq/client-sdk-go/config"
	"github.com/momentohq/client-sdk-go/momento"
	"log"
	"time"
)

func MustCreateMomentoClients(credProvider auth.CredentialProvider) (momento.CacheClient, momento.TopicClient) {
	const defaultTtl = 5 * time.Minute
	c, err := momento.NewCacheClient(
		config.LaptopLatest(),
		credProvider,
		defaultTtl,
	)
	if err != nil {
		log.Fatal("failed to initialize momento cache client", err)
	}

	tc, err := momento.NewTopicClient(config.TopicsDefault(), credProvider)
	if err != nil {
		log.Fatal("failed to initialize momento topic client", err)
	}
	return c, tc
}
