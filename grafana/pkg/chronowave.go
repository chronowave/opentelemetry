package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/chronowave/chronowave/ssql/parser"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// newChronoWaveDatasource returns datasource.ServeOpts.
func newChronoWaveDatasource() datasource.ServeOpts {
	// creates a instance manager for your plugin. The function passed
	// into `NewInstanceManger` is called when the instance is created
	// for the first time or when a datasource configuration changed.
	im := datasource.NewInstanceManager(newChronoWaveInstance)
	ds := &ChronoWaveDatasource{im: im}

	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

// ChronoWaveDatasource is an example datasource used to scaffold
// new datasource plugins with an backend.
type ChronoWaveDatasource struct {
	// The instance manager can help with lifecycle management
	// of datasource instances in plugins. It's not a requirements
	// but a best practice that we recommend that you follow.
	im instancemgmt.InstanceManager
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifer).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (cwd *ChronoWaveDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	log.DefaultLogger.Info("Chronowave QueryData", "request", req)

	instance, err := cwd.im.Get(req.PluginContext)
	if err != nil {
		return nil, err
	}

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for i, q := range req.Queries {
		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = cwd.query(ctx, instance.(*instanceSettings), i, q)
	}

	return response, nil
}

type model struct {
	Query     string `json:"ssql"`
	Timeframe string `json:"timeframe"`
	Format    string `json:"format"`
}

func (cwd *ChronoWaveDatasource) query(ctx context.Context, cw *instanceSettings, i int, query backend.DataQuery) backend.DataResponse {
	// Unmarshal the json into our queryModel
	var m model

	response := backend.DataResponse{}

	response.Error = json.Unmarshal(query.JSON, &m)
	if response.Error != nil {
		return response
	}

	// Log a warning if `Format` is empty.
	if m.Format == "" {
		log.DefaultLogger.Warn("format is empty. defaulting to time series")
	}

	if len(m.Query) == 0 {
		response.Error = errors.New("query is empty")
		return response
	}

	if len(m.Timeframe) == 0 {
		response.Error = errors.New("timeframe json path is empty")
		return response
	}

	_, errs := parser.Parse(m.Query)
	if len(errs) > 0 {
		msg := strings.Builder{}
		msg.WriteString("syntax error: ")
		for _, e := range errs {
			msg.WriteString(fmt.Sprintf("line %d column %d err: %s\n", e.Line, e.Column, e.Message))
		}

		response.Error = errors.New(msg.String())
		return response
	}

	modified := modifyTimeframe(m.Query, m.Timeframe, query.TimeRange)
	resp, err := request(ctx, cw.Url, modified)
	if err != nil {
		response.Error = err
		return response
	}

	var rs []map[string]interface{}

	err = json.Unmarshal(resp, &rs)
	if err != nil {
		response.Error = err
		return response
	}

	// create data frame response
	frame := data.NewFrame("chronowave_" + strconv.FormatInt(int64(i), 10))
	frame.RefID = query.RefID

	if len(rs) > 0 {
		fields := make([]struct {
			name   string
			int    []int64
			float  []float64
			string []string
		}, len(rs[0]))

		i := 0
		for k := range rs[0] {
			fields[i].name = k
			i++
		}

		for _, m := range rs {
			for i := range fields {
				v := m[fields[i].name]

				switch v.(type) {
				case int64:
					fields[i].int = append(fields[i].int, v.(int64))
				case float64:
					fields[i].float = append(fields[i].float, v.(float64))
				case string:
					fields[i].string = append(fields[i].string, v.(string))
				default:
					if d, err := json.Marshal(v); err == nil {
						fields[i].string = append(fields[i].string, string(d))
					}
				}
			}
		}

		for _, f := range fields {
			if len(f.string) > 0 {
				frame.Fields = append(frame.Fields, data.NewField(f.name, nil, f.string))
			} else if len(f.int) > 0 {
				frame.Fields = append(frame.Fields, data.NewField(f.name, nil, f.int))
			} else if len(f.float) > 0 {
				frame.Fields = append(frame.Fields, data.NewField(f.name, nil, f.float))
			}
		}
	}

	// add the frames to the response
	response.Frames = append(response.Frames, frame)

	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (cwd *ChronoWaveDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	rurl, err := url.Parse(req.PluginContext.DataSourceInstanceSettings.URL)
	if err == nil {
		rurl.Path = path.Join(rurl.Path, "health")
		_, err = request(ctx, rurl.String(), "")
	}

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	if err != nil {
		status = backend.HealthStatusError
		message = err.Error()
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

type instanceSettings struct {
	Url string `json:"url"`
}

func newChronoWaveInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	rurl, err := url.Parse(setting.URL)
	if err != nil {
		return nil, err
	}
	rurl.Path = path.Join(rurl.Path, "query")

	return &instanceSettings{Url: rurl.String()}, nil
}

func (s *instanceSettings) Dispose() {
	// Called before creatinga a new instance to allow plugin authors
	// to cleanup.
}

func request(ctx context.Context, url, query string) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", url, strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return ioutil.ReadAll(resp.Body)
	}

	return nil, errors.New("unexpected http response code " + strconv.FormatInt(int64(resp.StatusCode), 10))
}

func modifyTimeframe(original, timeframe string, tr backend.TimeRange) string {
	norm := strings.ToLower(original)
	if strings.Contains(norm, " timeframe") || strings.Contains(norm, " key") {
		return original
	}
	off := strings.Index(norm, " where ")
	if off <= 0 {
		return original
	}

	off += 7
	// Jaeger timestamp is microsecond
	from, to := strconv.FormatInt(tr.From.UnixNano()/1000, 10), strconv.FormatInt(tr.To.UnixNano()/1000, 10)
	return original[:off] + "[" + timeframe + " timeframe(" + from + "," + to + ")]" + original[off:]
}
