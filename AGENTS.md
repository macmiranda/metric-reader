# Agent Instructions for metric-reader

This is **alpha (pre-1.0)** software. Breaking changes are acceptable and backwards compatibility does not need to be maintained.

## Breaking Changes

### PROMETHEUS_QUERY replaces METRIC_NAME + LABEL_FILTERS

The `METRIC_NAME` and `LABEL_FILTERS` environment variables (and their corresponding `metric_name` / `label_filters` config file keys) have been removed and replaced with a single `PROMETHEUS_QUERY` field.

**Before:**
```toml
metric_name = "up"
label_filters = 'job="prometheus",instance="localhost:9090"'
```

**After:**
```toml
prometheus_query = 'up{job="prometheus",instance="localhost:9090"}'
```

This change supports full PromQL expressions, including:
- Simple metric names: `up`
- Metric names with label filters: `up{job="prometheus"}`
- Functions and operators: `rate(http_requests_total[5m])`
- Aggregations: `avg(node_memory_MemAvailable_bytes) / avg(node_memory_MemTotal_bytes) * 100`

The query result must be a scalar or single-element vector for threshold comparison to work correctly.
