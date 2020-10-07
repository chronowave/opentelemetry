import { DataSourceInstanceSettings } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';
import { ChronoWaveDataSourceOptions, ChronowaveQuery } from './types';

export class DataSource extends DataSourceWithBackend<ChronowaveQuery, ChronoWaveDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ChronoWaveDataSourceOptions>) {
    super(instanceSettings);
  }
}
