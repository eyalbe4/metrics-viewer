package commands

import (
	"fmt"
	"github.com/eldada/metrics-viewer/provider"
	"github.com/jfrog/jfrog-cli-core/artifactory/commands"
	"github.com/jfrog/jfrog-cli-core/plugins/components"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var FileFlag = components.StringFlag{
	Name:        "file",
	Description: "log file with the open metrics format",
}

var UrlFlag = components.StringFlag{
	Name:        "url",
	Description: "url endpoint to use to get metrics",
}

var UserFlag = components.StringFlag{
	Name:        "user",
	Description: "username for url requiring authentication",
}

var PasswordFlag = components.StringFlag{
	Name:        "password",
	Description: "password for url requiring authentication",
}

var ArtifactoryFlag = components.BoolFlag{
	Name:         "artifactory",
	Description:  "call Artifactory to get the metrics",
	DefaultValue: false,
}

var ServerFlag = components.StringFlag{
	Name:        "server",
	Description: "Artifactory server ID to call when --artifactory is given (uses current by default)",
}

var IntervalFlag = components.StringFlag{
	Name:         "interval",
	Description:  "scraping interval in seconds",
	DefaultValue: "5",
}

var FilterFlag = components.StringFlag{
	Name:        "filter",
	Description: "regular expression to use for filtering the metrics",
}

var AggregateIgnoreLabelsFlag = components.StringFlag{
	Name:         "aggregate-ignore-labels",
	Description:  "comma delimited list of labels to ignore when aggregating metrics. Use 'ALL' or 'NONE' to ignore all or none of the labels.",
	DefaultValue: "start,end,status",
}

func getCommonFlags() []components.Flag {
	return []components.Flag{
		FileFlag,
		UrlFlag,
		UserFlag,
		PasswordFlag,
		ArtifactoryFlag,
		ServerFlag,
		IntervalFlag,
		FilterFlag,
		AggregateIgnoreLabelsFlag,
	}
}

type commonConfiguration struct {
	file                  string
	urlMetricsFetcher     provider.UrlMetricsFetcher
	interval              time.Duration
	filter                *regexp.Regexp
	aggregateIgnoreLabels provider.StringSet
}

func (c commonConfiguration) UrlMetricsFetcher() provider.UrlMetricsFetcher {
	return c.urlMetricsFetcher
}

func (c commonConfiguration) File() string {
	return c.file
}

func (c commonConfiguration) Interval() time.Duration {
	return c.interval
}

func (c commonConfiguration) Filter() *regexp.Regexp {
	return c.filter
}

func (c commonConfiguration) AggregateIgnoreLabels() provider.StringSet {
	return c.aggregateIgnoreLabels
}

func (c commonConfiguration) String() string {
	return fmt.Sprintf("file: '%s', %s, interval: %s, filter: %s",
		c.file, c.urlMetricsFetcher, c.interval, c.filter.String())
}

func parseCommonConfig(c *components.Context) (*commonConfiguration, error) {
	conf := commonConfiguration{
		file: c.GetStringFlagValue("file"),
	}
	url := c.GetStringFlagValue("url")
	callArtifactory := c.GetBoolFlagValue("artifactory")

	countInputFlags := 0
	if conf.file != "" {
		countInputFlags++
	}
	if url != "" {
		countInputFlags++
	}
	if callArtifactory {
		countInputFlags++
	}
	if countInputFlags == 0 && os.Getenv("MOCK_METRICS_DATA") == "" {
		return nil, fmt.Errorf("one flag is required: --file | --url | --artifactory")
	}
	if countInputFlags > 1 {
		return nil, fmt.Errorf("only one flag is required: --file | --url | --artifactory")
	}

	if conf.file != "" {
		f, err := os.Open(conf.file)
		if err != nil {
			return nil, fmt.Errorf("could not open file %s: %w", conf.file, err)
		}
		_ = f.Close()
	}

	if callArtifactory {
		serverId := c.GetStringFlagValue("server")
		rtDetails, err := commands.GetConfig(serverId, false)
		if err != nil {
			msg := fmt.Sprintf("could not load configuration for Artifactory server %s", serverId)
			if serverId == "" {
				msg = "could not load configuration for current Artifactory server"
			}
			return nil, fmt.Errorf("%s; cause: %w", msg, err)
		}
		conf.urlMetricsFetcher, err = provider.NewArtifactoryMetricsFetcher(rtDetails)
		if err != nil {
			return nil, fmt.Errorf("could not initiate metrics fetcher from Artifactory; cause: %w", err)
		}
	}

	if url != "" {
		username := c.GetStringFlagValue("user")
		password := c.GetStringFlagValue("password")
		conf.urlMetricsFetcher = provider.NewUrlMetricsFetcher(url, username, password)
	}

	var flagValue string

	flagValue = c.GetStringFlagValue("interval")
	intValue, err := strconv.ParseInt(flagValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interval value: %s; cause: %w", flagValue, err)
	}
	if intValue <= 0 {
		return nil, fmt.Errorf("interval value must be positive; got: %d", intValue)
	}
	conf.interval = time.Duration(intValue) * time.Second

	conf.filter = regexp.MustCompile(".*")
	flagValue = c.GetStringFlagValue("filter")
	if flagValue != "" {
		conf.filter, err = regexp.Compile(flagValue)
		if err != nil {
			return nil, fmt.Errorf("invalid filter expression; cause: %w", err)
		}
	}

	flagValue = c.GetStringFlagValue("aggregate-ignore-labels")
	conf.aggregateIgnoreLabels = provider.StringSet{}
	if flagValue != "" {
		conf.aggregateIgnoreLabels.Add(strings.Split(flagValue, ",")...)
	}

	return &conf, nil
}
