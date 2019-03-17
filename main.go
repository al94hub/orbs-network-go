package main

import (
	"flag"
	"fmt"
	"github.com/orbs-network/orbs-network-go/bootstrap"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
)

func getLogger(path string, silent bool, cfg config.NodeConfig) log.BasicLogger {
	if path == "" {
		path = "./orbs-network.log"
	}

	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	fileWriter := log.NewTruncatingFileWriter(logFile, cfg.LoggerFileTruncationInterval())
	outputs := []log.Output{
		log.NewFormattingOutput(fileWriter, log.NewJsonFormatter()),
	}

	if !silent {
		outputs = append(outputs, log.NewFormattingOutput(os.Stdout, log.NewHumanReadableFormatter()))
	}

	if cfg.LoggerHttpEndpoint() != "" {
		customJSONFormatter := log.NewJsonFormatter().WithTimestampColumn("@timestamp")
		bulkSize := int(cfg.LoggerBulkSize())
		if bulkSize == 0 {
			bulkSize = 100
		}

		outputs = append(outputs, log.NewBulkOutput(log.NewHttpWriter(cfg.LoggerHttpEndpoint()), customJSONFormatter, bulkSize))
	}

	logger := log.GetLogger().WithTags(
		log.VirtualChainId(cfg.VirtualChainId()),
	).WithOutput(outputs...)

	conditionalFilter := log.NewConditionalFilter(false, nil)

	if !cfg.LoggerFullLog() {
		conditionalFilter = log.NewConditionalFilter(true, log.Or(log.OnlyErrors(), log.OnlyMetrics()))
	}

	return logger.WithFilters(conditionalFilter)
}

func getConfig(configFiles config.ArrayFlags, httpAddress string) (config.NodeConfig, error) {
	cfg := config.ForProduction("")

	if len(configFiles) != 0 {
		for _, configFile := range configFiles {
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return nil, errors.Errorf("could not open config file: %s", err)
			}

			contents, err := ioutil.ReadFile(configFile)
			if err != nil {
				return nil, err
			}

			cfg, err = cfg.MergeWithFileConfig(string(contents))

			if err != nil {
				return nil, err
			}
		}
	}

	cfg.SetString(config.HTTP_ADDRESS, httpAddress)

	return cfg, nil
}

func main() {
	httpAddress := flag.String("listen", ":8080", "ip address and port for http server")
	silentLog := flag.Bool("silent", false, "disable output to stdout")
	pathToLog := flag.String("log", "", "path/to/node.log")
	version := flag.Bool("version", false, "returns information about version")

	var configFiles config.ArrayFlags
	flag.Var(&configFiles, "config", "path/to/config.json")

	flag.Parse()

	if *version {
		fmt.Println(config.GetVersion())
		return
	}

	cfg, err := getConfig(configFiles, *httpAddress)
	if err != nil {
		fmt.Printf("%s \n", err)
		os.Exit(1)
	}

	logger := getLogger(*pathToLog, *silentLog, cfg)

	bootstrap.NewNode(
		cfg,
		logger,
	).WaitUntilShutdown()
}
