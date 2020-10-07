package main

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chronowave/chronowave/embed"
	"github.com/hashicorp/go-hclog"
	"github.com/labstack/echo/v4"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/plugin/storage/es/spanstore/dbmodel"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

const (
	timestamp = "/startTime"
)

var (
	keys = []string{"/traceID", "/spanID"}
)

type cwPlugin struct {
	store *WaveRider
}

func (p *cwPlugin) SpanReader() spanstore.Reader {
	return p.store
}

func (p *cwPlugin) SpanWriter() spanstore.Writer {
	return p.store
}

func (p *cwPlugin) DependencyReader() dependencystore.Reader {
	return p.store
}

type WaveRider struct {
	logger            hclog.Logger
	stream            *embed.WaveStream
	echo              *echo.Echo
	from              dbmodel.FromDomain
	to                dbmodel.ToDomain
	once              sync.Once
	serviceOperations map[string]map[string]bool
	rwLock            sync.RWMutex
}

func newWaveRider(logger hclog.Logger, conf *conf) *WaveRider {
	wave := embed.NewWave(conf.dir, timestamp, keys)
	return &WaveRider{
		logger:            logger,
		stream:            wave,
		echo:              startEcho(wave, conf.port),
		from:              dbmodel.FromDomain{},
		to:                dbmodel.ToDomain{},
		serviceOperations: map[string]map[string]bool{},
	}
}

func (wr *WaveRider) Close() {
	wr.echo.Shutdown(context.Background())
	wr.stream.Close()
}

func (wr *WaveRider) WriteSpan(ctx context.Context, span *model.Span) error {
	wr.updateSvcOp(span)

	jsonSpan := wr.from.FromDomainEmbedProcess(span)
	json, err := json.Marshal(jsonSpan)
	if err == nil {
		err = wr.stream.OnNewDocument(json)
	}

	return err
}

func (wr *WaveRider) updateSvcOp(span *model.Span) {
	wr.rwLock.Lock()
	defer wr.rwLock.Unlock()
	op, ok := wr.serviceOperations[span.Process.ServiceName]
	if !ok {
		op = map[string]bool{}
		wr.serviceOperations[span.Process.ServiceName] = op
	}

	op[span.OperationName] = true
}

// GetTrace retrieves the trace with a given id.
//
// If no spans are stored for this trace, it returns ErrTraceNotFound.
func (wr *WaveRider) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	sb := strings.Builder{}
	sb.WriteString("FIND $s WHERE [/traceID KEY('")
	sb.WriteString(traceID.String())
	sb.WriteString("')] [$s /]")
	jdoc, err := wr.stream.Query(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	var rs []struct{ S *dbmodel.Span }
	err = json.Unmarshal(jdoc, &rs)
	if err != nil {
		return nil, err
	}

	spans := make([]*model.Span, len(rs))
	for i, v := range rs {
		if spans[i], err = wr.to.SpanToDomain(v.S); err != nil {
			return nil, err
		}
	}

	return &model.Trace{Spans: spans}, nil
}

// GetServices returns all service names known to the backend from spans
// within its retention period.
func (wr *WaveRider) GetServices(ctx context.Context) ([]string, error) {
	wr.once.Do(wr.queryService)

	svc := make([]string, len(wr.serviceOperations))
	i := 0
	for k := range wr.serviceOperations {
		svc[i] = k
		i++
	}

	return svc, nil
}

// GetOperations returns all operation names for a given service
// known to the backend from spans within its retention period.
func (wr *WaveRider) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	wr.once.Do(wr.queryService)

	wr.rwLock.RLock()
	ops := wr.serviceOperations[query.ServiceName]
	wr.rwLock.RUnlock()

	retMe := make([]spanstore.Operation, len(ops))
	i := 0
	for k := range ops {
		retMe[i].Name = k
		i++
	}

	return retMe, nil
}

// FindTraces returns all traces matching query parameters. There's currently
// an implementation-dependent abiguity whether all query filters (such as
// multiple tags) must apply to the same span within a trace, or can be satisfied
// by different spans.
//
// If no matching traces are found, the function returns (nil, nil).
func (wr *WaveRider) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	qry, min, max := buildTraceIdQuery(query)
	jdoc, err := wr.stream.Query(ctx, qry)
	if err != nil {
		return nil, err
	}

	var rs []struct{ Tid string }
	err = json.Unmarshal(jdoc, &rs)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	if len(rs) > query.NumTraces {
		rs = rs[:query.NumTraces]
	}

	sb := strings.Builder{}
	sb.WriteString("FIND $s WHERE [$s /] [/startTime TIMEFRAME(")
	sb.WriteString(strconv.FormatInt(min, 10))
	sb.WriteString(",")
	sb.WriteString(strconv.FormatInt(max, 10))
	sb.WriteString(")] [/traceID IN(")

	comma := ""
	for _, v := range rs {
		sb.WriteString(comma)
		sb.WriteByte('\'')
		sb.WriteString(v.Tid)
		sb.WriteByte('\'')
		comma = ","
	}
	sb.WriteString(")]")

	jdoc, err = wr.stream.Query(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	var spans []struct{ S *dbmodel.Span }
	err = json.Unmarshal(jdoc, &spans)
	if err != nil {
		return nil, err
	}

	traces := make(map[string]*model.Trace, len(spans))
	for _, v := range spans {
		if span, err := wr.to.SpanToDomain(v.S); err == nil {
			traceid := span.TraceID.String()

			trace, ok := traces[traceid]
			if !ok {
				trace = &model.Trace{}
				traces[traceid] = trace
			}
			trace.Spans = append(trace.Spans, span)
		}
	}

	retMe := make([]*model.Trace, len(traces))

	i := 0
	for _, v := range traces {
		retMe[i] = v
		i++
	}

	return retMe, nil
}

// FindTraceIDs does the same search as FindTraces, but returns only the list
// of matching trace IDs.
//
// If no matching traces are found, the function returns (nil, nil).
func (wr *WaveRider) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	qry, _, _ := buildTraceIdQuery(query)
	jdoc, err := wr.stream.Query(ctx, qry)
	if err != nil {
		return nil, err
	}

	var rs []struct{ Tid string }
	err = json.Unmarshal(jdoc, &rs)
	if err != nil || len(rs) == 0 {
		return nil, err
	}

	retMe := make([]model.TraceID, len(rs))
	for i, v := range rs {
		if retMe[i], err = model.TraceIDFromString(v.Tid); err != nil {
			return nil, err
		}
	}

	return retMe, nil
}

func (wr *WaveRider) GetDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	sb := strings.Builder{}
	sb.WriteString("FIND $ref, $sid, $svc WHERE [$ref /references][$sid /spanID][$svc /process/serviceName]")
	sb.WriteString("[/startTime TIMEFRAME(")
	sb.WriteString(strconv.FormatInt(endTs.Add(-1*lookback).UnixNano()/1000, 10))
	sb.WriteString(",")
	sb.WriteString(strconv.FormatInt(endTs.UnixNano()/1000, 10))
	sb.WriteString(")]")

	jdoc, err := wr.stream.Query(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	var rs []struct {
		Ref []dbmodel.Reference
		Sid string
		Svc string
	}
	err = json.Unmarshal(jdoc, &rs)
	if err != nil {
		return nil, err
	}

	spanMap := make(map[string]int, len(rs))
	for i, s := range rs {
		spanMap[s.Sid] = i
	}

	deps := make(map[string]int, len(rs))
	retMe := make([]model.DependencyLink, 0, len(rs))
	for _, v := range rs {
		var pid string
		for _, r := range v.Ref {
			if r.RefType == dbmodel.ChildOf {
				pid = string(r.SpanID)
			}
		}
		if len(pid) == 0 {
			continue
		}

		if parent, ok := spanMap[pid]; ok {
			if strings.Compare(rs[parent].Svc, v.Svc) == 0 {
				continue
			}
			depKey := rs[parent].Svc + "&&&" + v.Svc
			if i, ok := deps[depKey]; !ok {
				deps[depKey] = len(retMe)
				retMe = append(retMe, model.DependencyLink{
					Parent:    rs[parent].Svc,
					Child:     v.Svc,
					CallCount: 1,
				})
			} else {
				retMe[i].CallCount++
			}
		}
	}

	return retMe, nil
}

func (wr *WaveRider) queryService() {
	sb := strings.Builder{}
	sb.WriteString("FIND $svc, $op WHERE [$svc /process/serviceName][$op /operationName]")
	sb.WriteString("[/startTime TIMEFRAME(")
	sb.WriteString(strconv.FormatInt(time.Now().Add(-336*time.Hour).UnixNano()/1000, 10))
	sb.WriteString(",")
	sb.WriteString(strconv.FormatInt(time.Now().UnixNano()/1000, 10))
	sb.WriteString(")]")

	jdoc, err := wr.stream.Query(context.Background(), sb.String())
	if err != nil {
		return
	}

	var rs []struct {
		Svc string
		Op  string
	}
	err = json.Unmarshal(jdoc, &rs)
	if err != nil {
		return
	}

	wr.rwLock.Lock()
	defer wr.rwLock.Unlock()
	for _, v := range rs {
		op, ok := wr.serviceOperations[v.Svc]
		if !ok {
			op = map[string]bool{}
			wr.serviceOperations[v.Svc] = op
		}
		op[v.Op] = true
	}
}

func buildTraceIdQuery(query *spanstore.TraceQueryParameters) (string, int64, int64) {
	min, max := int64(0), int64(math.MaxInt64)
	if !query.StartTimeMin.IsZero() {
		min = query.StartTimeMin.UnixNano() / int64(1000)
	}
	if !query.StartTimeMax.IsZero() {
		max = query.StartTimeMax.UnixNano() / int64(1000)
	}

	sb := strings.Builder{}
	sb.WriteString("FIND $tid, $st WHERE [$tid /traceID] [$st /startTime TIMEFRAME(")
	sb.WriteString(strconv.FormatInt(min, 10))
	sb.WriteString(",")
	sb.WriteString(strconv.FormatInt(max, 10))
	sb.WriteString(")]")

	if len(query.ServiceName) > 0 {
		sb.WriteString("[/process/serviceName CONTAIN('^")
		sb.WriteString(query.ServiceName)
		sb.WriteString("$')]")
	}

	if len(query.OperationName) > 0 {
		sb.WriteString("[/operationName CONTAIN('^")
		sb.WriteString(query.OperationName)
		sb.WriteString("$')]")
	}

	// https://github.com/jaegertracing/jaeger/blob/master/plugin/storage/es/spanstore/dbmodel/from_domain.go#L55
	if query.DurationMin != 0 && query.DurationMax != 0 {
		sb.WriteString("[/duration BETWEEN(")
		sb.WriteString(strconv.FormatInt(query.DurationMin.Nanoseconds()/1000, 10))
		sb.WriteString(",")
		sb.WriteString(strconv.FormatInt(query.DurationMax.Nanoseconds()/1000, 10))
		sb.WriteString(")]")
	} else if query.DurationMin > 0 {
		sb.WriteString("[/duration GE(")
		sb.WriteString(strconv.FormatInt(query.DurationMin.Nanoseconds()/1000, 10))
		sb.WriteString(")]")
	} else if query.DurationMax > 0 {
		sb.WriteString("[/duration LE(")
		sb.WriteString(strconv.FormatInt(query.DurationMax.Nanoseconds()/1000, 10))
		sb.WriteString(")]")
	}

	if len(query.Tags) > 0 {
		for k, v := range query.Tags {
			sb.WriteString("[/tags ")
			sb.WriteString("[/key ")
			sb.WriteString(" CONTAIN('")
			sb.WriteString(k)
			sb.WriteString("')]")
			sb.WriteString("[/value ")
			sb.WriteString(" CONTAIN('")
			sb.WriteString(v)
			sb.WriteString("')]")
			sb.WriteString("]")
		}
	}

	sb.WriteString("order-by $st desc")

	return sb.String(), min, max
}
