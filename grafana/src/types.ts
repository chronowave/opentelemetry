import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface ChronowaveQuery extends DataQuery {
  ssql: string;
  timeframe: string;
}

export const defaultQuery: Partial<ChronowaveQuery> = {
  ssql: ``,
  timeframe: ``,
};

/**
 * These are options configured for each DataSource instance
 */
export interface ChronoWaveDataSourceOptions extends DataSourceJsonData {
  url?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  apiKey?: string;
}
