import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './DataSource';
import { ConfigEditor } from './ConfigEditor';
import { QueryEditor } from './QueryEditor';
import { ChronowaveQuery, ChronoWaveDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ChronowaveQuery, ChronoWaveDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
