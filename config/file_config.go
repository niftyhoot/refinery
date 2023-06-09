package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// In order to be able to unmarshal "15s" etc. into time.Duration, we need to
// define a new type and implement MarshalText and UnmarshalText.
type Duration time.Duration

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

type fileConfig struct {
	mainConfig    *configContents
	mainHash      string
	rulesConfig   *V2SamplerConfig
	rulesHash     string
	opts          *CmdEnv
	callbacks     []func()
	errorCallback func(error)
	done          chan struct{}
	ticker        *time.Ticker
	mux           sync.RWMutex
	lastLoadTime  time.Time
}

type configContents struct {
	General              GeneralConfig             `yaml:"General"`
	Network              NetworkConfig             `yaml:"Network"`
	AccessKeys           AccessKeyConfig           `yaml:"AccessKeys"`
	Telemetry            RefineryTelemetryConfig   `yaml:"RefineryTelemetry"`
	Traces               TracesConfig              `yaml:"Traces"`
	Debugging            DebuggingConfig           `yaml:"Debugging"`
	Logger               LoggerConfig              `yaml:"Logger"`
	HoneycombLogger      HoneycombLoggerConfig     `yaml:"HoneycombLogger"`
	StdoutLogger         StdoutLoggerConfig        `yaml:"StdoutLogger"`
	PrometheusMetrics    PrometheusMetricsConfig   `yaml:"PrometheusMetrics"`
	LegacyMetrics        LegacyMetricsConfig       `yaml:"LegacyMetrics"`
	OTelMetrics          OTelMetricsConfig         `yaml:"OTelMetrics"`
	PeerManagement       PeerManagementConfig      `yaml:"PeerManagement"`
	RedisPeerManagement  RedisPeerManagementConfig `yaml:"RedisPeerManagement"`
	Collection           CollectionConfig          `yaml:"Collection"`
	BufferSizes          BufferSizeConfig          `yaml:"BufferSizes"`
	Specialized          SpecializedConfig         `yaml:"Specialized"`
	IDFieldNames         IDFieldsConfig            `yaml:"IDFields"`
	GRPCServerParameters GRPCServerParameters      `yaml:"GRPCServerParameters"`
	SampleCache          SampleCacheConfig         `yaml:"SampleCache"`
	StressRelief         StressReliefConfig        `yaml:"StressRelief"`
}

type GeneralConfig struct {
	ConfigurationVersion int      `yaml:"ConfigurationVersion"`
	MinRefineryVersion   string   `yaml:"MinRefineryVersion" default:"v2.0"`
	DatasetPrefix        string   `yaml:"DatasetPrefix" `
	ConfigReloadInterval Duration `yaml:"ConfigReloadInterval" default:"5m"`
}

type NetworkConfig struct {
	ListenAddr     string `yaml:"ListenAddr" default:"0.0.0.0:8080" cmdenv:"HTTPListenAddr"`
	PeerListenAddr string `yaml:"PeerListenAddr" default:"0.0.0.0:8081" cmdenv:"PeerListenAddr"`
	HoneycombAPI   string `yaml:"HoneycombAPI" default:"https://api.honeycomb.io" cmdenv:"HoneycombAPI"`
}

type AccessKeyConfig struct {
	ReceiveKeys          []string `yaml:"ReceiveKeys" default:"[]"`
	AcceptOnlyListedKeys bool     `yaml:"AcceptOnlyListedKeys"`
}

type RefineryTelemetryConfig struct {
	AddRuleReasonToTrace   bool `yaml:"AddRuleReasonToTrace"`
	AddSpanCountToRoot     bool `yaml:"AddSpanCountToRoot"`
	AddHostMetadataToTrace bool `yaml:"AddHostMetadataToTrace"`
}

type TracesConfig struct {
	SendDelay    Duration `yaml:"SendDelay" default:"2s"`
	BatchTimeout Duration `yaml:"BatchTimeout" default:"100ms"`
	TraceTimeout Duration `yaml:"TraceTimeout" default:"60s"`
	MaxBatchSize uint     `yaml:"MaxBatchSize" default:"500"`
	SendTicker   Duration `yaml:"SendTicker" default:"100ms"`
}

type DebuggingConfig struct {
	DebugServiceAddr      string   `yaml:"DebugServiceAddr"`
	QueryAuthToken        string   `yaml:"QueryAuthToken" cmdenv:"QueryAuthToken"`
	AdditionalErrorFields []string `yaml:"AdditionalErrorFields" default:"[\"trace.span_id\"]"`
	DryRun                bool     `yaml:"DryRun" `
}

type LoggerConfig struct {
	Type  string `yaml:"Type" default:"stdout"`
	Level Level  `yaml:"Level" default:"Warn"`
}

type HoneycombLoggerConfig struct {
	APIHost           string `yaml:"APIHost" default:"https://api.honeycomb.io"`
	APIKey            string `yaml:"APIKey" cmdenv:"HoneycombLoggerAPIKey,HoneycombAPIKey"`
	Dataset           string `yaml:"Dataset" default:"Refinery Logs"`
	SamplerEnabled    bool   `yaml:"SamplerEnabled" `
	SamplerThroughput int    `yaml:"SamplerThroughput" default:"5"`
}

type StdoutLoggerConfig struct {
	Structured bool `yaml:"Structured" default:"true"`
}

type PrometheusMetricsConfig struct {
	Enabled    bool   `yaml:"Enabled" default:"false"`
	ListenAddr string `yaml:"ListenAddr"`
}

type LegacyMetricsConfig struct {
	Enabled           bool     `yaml:"Enabled" default:"false"`
	APIHost           string   `yaml:"APIHost" default:"https://api.honeycomb.io"`
	APIKey            string   `yaml:"APIKey" cmdenv:"LegacyMetricsAPIKey,HoneycombAPIKey"`
	Dataset           string   `yaml:"Dataset" default:"Refinery Metrics"`
	ReportingInterval Duration `yaml:"ReportingInterval" default:"30s"`
}

type OTelMetricsConfig struct {
	Enabled           bool     `yaml:"Enabled" default:"false"`
	APIHost           string   `yaml:"APIHost" default:"https://api.honeycomb.io"`
	APIKey            string   `yaml:"APIKey" cmdenv:"OTelMetricsAPIKey,HoneycombAPIKey"`
	Dataset           string   `yaml:"Dataset" default:"Refinery Metrics"`
	Compression       string   `yaml:"Compression" default:"gzip"`
	ReportingInterval Duration `yaml:"ReportingInterval" default:"30s"`
}

type PeerManagementConfig struct {
	Type                    string   `yaml:"Type" default:"file"`
	Identifier              string   `yaml:"Identifier"`
	IdentifierInterfaceName string   `yaml:"IdentifierInterfaceName"`
	UseIPV6Identifier       bool     `yaml:"UseIPV6Identifier"`
	Peers                   []string `yaml:"Peers"`
}

type RedisPeerManagementConfig struct {
	Host           string   `yaml:"Host" cmdenv:"RedisHost"`
	Username       string   `yaml:"Username" cmdenv:"RedisUsername"`
	Password       string   `yaml:"Password" cmdenv:"RedisPassword"`
	Prefix         string   `yaml:"Prefix" default:"refinery"`
	Database       int      `yaml:"Database"`
	UseTLS         bool     `yaml:"UseTLS" `
	UseTLSInsecure bool     `yaml:"UseTLSInsecure" `
	Timeout        Duration `yaml:"Timeout" default:"5s"`
}

type CollectionConfig struct {
	// CacheCapacity must be less than math.MaxInt32
	CacheCapacity int    `yaml:"CacheCapacity" default:"10_000"`
	MaxMemory     int    `yaml:"MaxMemory" default:"75"`
	MaxAlloc      uint64 `yaml:"MaxAlloc"`
}

type BufferSizeConfig struct {
	UpstreamBufferSize int `yaml:"UpstreamBufferSize" default:"10_000"`
	PeerBufferSize     int `yaml:"PeerBufferSize" default:"10_000"`
}

type SpecializedConfig struct {
	EnvironmentCacheTTL       Duration          `yaml:"EnvironmentCacheTTL" default:"1h"`
	CompressPeerCommunication bool              `yaml:"CompressPeerCommunication" default:"true"`
	AdditionalAttributes      map[string]string `yaml:"AdditionalAttributes" default:"{}"`
}

type IDFieldsConfig struct {
	TraceNames  []string `yaml:"TraceNames" default:"[\"trace.trace_id\",\"traceId\"]"`
	ParentNames []string `yaml:"ParentNames" default:"[\"trace.parent_id\",\"parentId\"]"`
}

// GRPCServerParameters allow you to configure the GRPC ServerParameters used
// by refinery's own GRPC server:
// https://pkg.go.dev/google.golang.org/grpc/keepalive#ServerParameters
type GRPCServerParameters struct {
	Enabled               bool     `yaml:"Enabled"`
	ListenAddr            string   `yaml:"ListenAddr" cmdenv:"GRPCListenAddr"`
	MaxConnectionIdle     Duration `yaml:"MaxConnectionIdle" default:"1m"`
	MaxConnectionAge      Duration `yaml:"MaxConnectionAge" default:"3m"`
	MaxConnectionAgeGrace Duration `yaml:"MaxConnectionAgeGrace" default:"1m"`
	KeepAlive             Duration `yaml:"KeepAlive" default:"1m"`
	KeepAliveTimeout      Duration `yaml:"KeepAliveTimeout" default:"20s"`
}

type SampleCacheConfig struct {
	KeptSize          uint     `yaml:"KeptSize" default:"10_000"`
	DroppedSize       uint     `yaml:"DroppedSize" default:"1_000_000"`
	SizeCheckInterval Duration `yaml:"SizeCheckInterval" default:"10s"`
}

type StressReliefConfig struct {
	Mode                      string   `yaml:"Mode" default:"never"`
	ActivationLevel           uint     `yaml:"ActivationLevel" default:"90"`
	DeactivationLevel         uint     `yaml:"DeactivationLevel" default:"75"`
	SamplingRate              uint64   `yaml:"SamplingRate" default:"1000"`
	MinimumActivationDuration Duration `yaml:"MinimumActivationDuration" default:"10s"`
	MinimumStartupDuration    Duration `yaml:"MinimumStartupDuration" default:"3s"`
}

type FileConfigError struct {
	ConfigLocation string
	ConfigFailures []string
	RulesLocation  string
	RulesFailures  []string
}

func (e *FileConfigError) Error() string {
	var msg strings.Builder
	if len(e.ConfigFailures) > 0 {
		msg.WriteString("Validation failed for config file ")
		msg.WriteString(e.ConfigLocation)
		msg.WriteString(":\n")
		for _, fail := range e.ConfigFailures {
			msg.WriteString("  ")
			msg.WriteString(fail)
			msg.WriteString("\n")
		}
	}
	if len(e.RulesFailures) > 0 {
		msg.WriteString("Validation failed for rules file ")
		msg.WriteString(e.RulesLocation)
		msg.WriteString(":\n")
		for _, fail := range e.RulesFailures {
			msg.WriteString("  ")
			msg.WriteString(fail)
			msg.WriteString("\n")
		}
	}
	return msg.String()
}

// newFileConfig does the work of creating and loading the start of a config object
// from the given arguments.
// It's used by both the main init as well as the reload code.
// In order to do proper validation, we actually read the file twice -- once into
// a map, and once into the actual config object.
func newFileConfig(opts *CmdEnv) (*fileConfig, error) {
	// If we're not validating, skip this part
	if !opts.NoValidate {
		cfgFails, err := validateConfig(opts.ConfigLocation)
		if err != nil {
			return nil, err
		}

		ruleFails, err := validateRules(opts.RulesLocation)
		if err != nil {
			return nil, err
		}

		if len(cfgFails) > 0 || len(ruleFails) > 0 {
			return nil, &FileConfigError{
				ConfigLocation: opts.ConfigLocation,
				ConfigFailures: cfgFails,
				RulesLocation:  opts.RulesLocation,
				RulesFailures:  ruleFails,
			}
		}
	}

	// Now load the files
	mainconf := &configContents{}
	mainhash, err := readConfigInto(mainconf, opts.ConfigLocation, opts)
	if err != nil {
		return nil, err
	}

	var rulesconf *V2SamplerConfig
	ruleshash, err := readConfigInto(&rulesconf, opts.RulesLocation, nil)
	if err != nil {
		return nil, err
	}

	cfg := &fileConfig{
		mainConfig:  mainconf,
		mainHash:    mainhash,
		rulesConfig: rulesconf,
		rulesHash:   ruleshash,
		opts:        opts,
	}

	return cfg, nil
}

// writeYAMLToFile renders the given data item to a YAML file
func writeYAMLToFile(data any, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// NewConfig creates a new Config object from the given arguments; if args is
// nil, it uses the command line arguments.
// It also dumps the config and rules to the given files, if specified, which
// will cause the program to exit.
func NewConfig(opts *CmdEnv, errorCallback func(error)) (Config, error) {
	cfg, err := newFileConfig(opts)
	// only exit if we have no config at all; if it fails validation, we'll
	// do the rest and return it anyway
	if err != nil && cfg == nil {
		return nil, err
	}

	if opts.WriteConfig != "" {
		if err := writeYAMLToFile(cfg.mainConfig, opts.WriteConfig); err != nil {
			fmt.Printf("Error writing config: %s\n", err)
			os.Exit(1)
		}
	}
	if opts.WriteRules != "" {
		if err := writeYAMLToFile(cfg.rulesConfig, opts.WriteRules); err != nil {
			fmt.Printf("Error writing rules: %s\n", err)
			os.Exit(1)
		}
	}
	if opts.WriteConfig != "" || opts.WriteRules != "" {
		os.Exit(0)
	}

	cfg.callbacks = make([]func(), 0)
	cfg.errorCallback = errorCallback

	go cfg.monitor()

	return cfg, err
}

func (f *fileConfig) monitor() {
	f.done = make(chan struct{})
	f.ticker = time.NewTicker(time.Duration(f.mainConfig.General.ConfigReloadInterval))
	for {
		select {
		case <-f.done:
			return
		case <-f.ticker.C:
			// reread the configs
			cfg, err := newFileConfig(f.opts)
			if err != nil {
				f.errorCallback(err)
				continue
			}

			// if nothing's changed, we're fine
			if f.mainHash == cfg.mainHash && f.rulesHash == cfg.rulesHash {
				continue
			}

			// otherwise, update our state and call the callbacks
			f.mux.Lock()
			f.mainConfig = cfg.mainConfig
			f.mainHash = cfg.mainHash
			f.rulesConfig = cfg.rulesConfig
			f.rulesHash = cfg.rulesHash
			for _, cb := range f.callbacks {
				cb()
			}
			f.mux.Unlock() // can't defer since the goroutine never ends
		}
	}
}

// Stop halts the monitor goroutine
func (f *fileConfig) Stop() {
	f.ticker.Stop()
	close(f.done)
	f.done = nil
}

func (f *fileConfig) RegisterReloadCallback(cb func()) {
	f.mux.Lock()
	defer f.mux.Unlock()

	f.callbacks = append(f.callbacks, cb)
}

func (f *fileConfig) GetListenAddr() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	_, _, err := net.SplitHostPort(f.mainConfig.Network.ListenAddr)
	if err != nil {
		return "", err
	}
	return f.mainConfig.Network.ListenAddr, nil
}

func (f *fileConfig) GetPeerListenAddr() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	_, _, err := net.SplitHostPort(f.mainConfig.Network.PeerListenAddr)
	if err != nil {
		return "", err
	}
	return f.mainConfig.Network.PeerListenAddr, nil
}

func (f *fileConfig) GetCompressPeerCommunication() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Specialized.CompressPeerCommunication
}

func (f *fileConfig) GetGRPCListenAddr() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	// GRPC listen addr is optional, only check value is valid if not empty
	if f.mainConfig.GRPCServerParameters.ListenAddr != "" {
		_, _, err := net.SplitHostPort(f.mainConfig.GRPCServerParameters.ListenAddr)
		if err != nil {
			return "", err
		}
	}
	return f.mainConfig.GRPCServerParameters.ListenAddr, nil
}

func (f *fileConfig) GetAPIKeys() ([]string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.AccessKeys.ReceiveKeys, nil
}

func (f *fileConfig) GetPeerManagementType() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PeerManagement.Type, nil
}

func (f *fileConfig) GetPeers() ([]string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PeerManagement.Peers, nil
}

func (f *fileConfig) GetRedisHost() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.Host, nil
}

func (f *fileConfig) GetRedisUsername() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.Username, nil
}

func (f *fileConfig) GetRedisPrefix() string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.Prefix
}

func (f *fileConfig) GetRedisPassword() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.Password, nil
}

func (f *fileConfig) GetRedisDatabase() int {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.Database
}

func (f *fileConfig) GetUseTLS() (bool, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.UseTLS, nil
}

func (f *fileConfig) GetUseTLSInsecure() (bool, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.RedisPeerManagement.UseTLSInsecure, nil
}

func (f *fileConfig) GetIdentifierInterfaceName() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PeerManagement.IdentifierInterfaceName, nil
}

func (f *fileConfig) GetUseIPV6Identifier() (bool, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PeerManagement.UseIPV6Identifier, nil
}

func (f *fileConfig) GetRedisIdentifier() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PeerManagement.Identifier, nil
}

func (f *fileConfig) GetHoneycombAPI() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Network.HoneycombAPI, nil
}

func (f *fileConfig) GetLoggerLevel() Level {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Logger.Level
}

func (f *fileConfig) GetLoggerType() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Logger.Type, nil
}

func (f *fileConfig) GetHoneycombLoggerConfig() (HoneycombLoggerConfig, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.HoneycombLogger, nil
}

// TODO: DEPRECATED
func (f *fileConfig) GetCollectorType() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return "InMemCollector", nil
}

func (f *fileConfig) GetAllSamplerRules() (*V2SamplerConfig, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	// This is probably good enough for debug; if not we can extend it.
	return f.rulesConfig, nil
}

// GetSamplerConfigForDestName returns the sampler config for the given
// destination (environment, or dataset in classic mode), as well as the name of
// the sampler type. If the specific destination is not found, it returns the
// default sampler config.
func (f *fileConfig) GetSamplerConfigForDestName(destname string) (any, string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	nameToUse := "__default__"
	if _, ok := f.rulesConfig.Samplers[destname]; ok {
		nameToUse = destname
	}

	err := errors.New("no sampler found and no default configured")
	name := "not found"
	var cfg any
	if sampler, ok := f.rulesConfig.Samplers[nameToUse]; ok {
		cfg, name = sampler.Sampler()
		if cfg != nil {
			err = nil
		}
	}
	return cfg, name, err
}

func (f *fileConfig) GetInMemCollectorCacheCapacity() (CollectionConfig, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Collection, nil
}

// TODO: REMOVE THIS
func (f *fileConfig) GetMetricsType() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return "", nil
}

func (f *fileConfig) GetLegacyMetricsConfig() LegacyMetricsConfig {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.LegacyMetrics
}

func (f *fileConfig) GetPrometheusMetricsConfig() PrometheusMetricsConfig {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.PrometheusMetrics
}

func (f *fileConfig) GetOTelMetricsConfig() OTelMetricsConfig {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.OTelMetrics
}

func (f *fileConfig) GetSendDelay() (time.Duration, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.Traces.SendDelay), nil
}

func (f *fileConfig) GetBatchTimeout() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.Traces.BatchTimeout)
}

func (f *fileConfig) GetTraceTimeout() (time.Duration, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.Traces.TraceTimeout), nil
}

func (f *fileConfig) GetMaxBatchSize() uint {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Traces.MaxBatchSize
}

func (f *fileConfig) GetUpstreamBufferSize() int {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.BufferSizes.UpstreamBufferSize
}

func (f *fileConfig) GetPeerBufferSize() int {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.BufferSizes.PeerBufferSize
}

func (f *fileConfig) GetSendTickerValue() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.Traces.SendTicker)
}

func (f *fileConfig) GetDebugServiceAddr() (string, error) {
	f.mux.RLock()
	defer f.mux.RUnlock()

	_, _, err := net.SplitHostPort(f.mainConfig.Debugging.DebugServiceAddr)
	if err != nil {
		return "", err
	}
	return f.mainConfig.Debugging.DebugServiceAddr, nil
}

func (f *fileConfig) GetIsDryRun() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Debugging.DryRun
}

// TODO: DEPRECATED
func (f *fileConfig) GetDryRunFieldName() string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return "meta.refinery.dryrun.kept"
}

func (f *fileConfig) GetAddHostMetadataToTrace() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Telemetry.AddHostMetadataToTrace
}

func (f *fileConfig) GetAddRuleReasonToTrace() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Telemetry.AddRuleReasonToTrace
}

func (f *fileConfig) GetEnvironmentCacheTTL() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.Specialized.EnvironmentCacheTTL)
}

func (f *fileConfig) GetDatasetPrefix() string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.General.DatasetPrefix
}

func (f *fileConfig) GetQueryAuthToken() string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Debugging.QueryAuthToken
}

func (f *fileConfig) GetGRPCMaxConnectionIdle() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.GRPCServerParameters.MaxConnectionIdle)
}

func (f *fileConfig) GetGRPCMaxConnectionAge() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.GRPCServerParameters.MaxConnectionAge)
}

func (f *fileConfig) GetGRPCMaxConnectionAgeGrace() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.GRPCServerParameters.MaxConnectionAgeGrace)
}

func (f *fileConfig) GetGRPCKeepAlive() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.GRPCServerParameters.KeepAlive)
}

func (f *fileConfig) GetGRPCKeepAliveTimeout() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.GRPCServerParameters.KeepAliveTimeout)
}

func (f *fileConfig) GetPeerTimeout() time.Duration {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return time.Duration(f.mainConfig.RedisPeerManagement.Timeout)
}

func (f *fileConfig) GetAdditionalErrorFields() []string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Debugging.AdditionalErrorFields
}

func (f *fileConfig) GetAddSpanCountToRoot() bool {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Telemetry.AddSpanCountToRoot
}

// TODO: DEPRECATE
func (f *fileConfig) GetCacheOverrunStrategy() string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return "impact"
}

func (f *fileConfig) GetSampleCacheConfig() SampleCacheConfig {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.SampleCache
}

func (f *fileConfig) GetStressReliefConfig() StressReliefConfig {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.StressRelief
}

func (f *fileConfig) GetTraceIdFieldNames() []string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.IDFieldNames.TraceNames
}

func (f *fileConfig) GetParentIdFieldNames() []string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.IDFieldNames.ParentNames
}

func (f *fileConfig) GetConfigMetadata() []ConfigMetadata {
	ret := make([]ConfigMetadata, 2)
	ret[0] = ConfigMetadata{
		Type:     "config",
		ID:       f.opts.ConfigLocation,
		Hash:     f.mainHash,
		LoadedAt: f.lastLoadTime.Format(time.RFC3339),
	}
	ret[1] = ConfigMetadata{
		Type:     "rules",
		ID:       f.opts.RulesLocation,
		Hash:     f.rulesHash,
		LoadedAt: f.lastLoadTime.Format(time.RFC3339),
	}
	return ret
}

func (f *fileConfig) GetAdditionalAttributes() map[string]string {
	f.mux.RLock()
	defer f.mux.RUnlock()

	return f.mainConfig.Specialized.AdditionalAttributes
}
