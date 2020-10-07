import defaults from 'lodash/defaults';

import React, { ChangeEvent, PureComponent } from 'react';
import { LegacyForms } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { defaultQuery, ChronoWaveDataSourceOptions, ChronowaveQuery } from './types';

const { FormField } = LegacyForms;

type Props = QueryEditorProps<DataSource, ChronowaveQuery, ChronoWaveDataSourceOptions>;

export class QueryEditor extends PureComponent<Props> {
  onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, ssql: event.target.value });
  };

  onTimeframeChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, timeframe: event.target.value });
  };

  render() {
    const query = defaults(this.props.query, defaultQuery);
    const { ssql } = query;

    return (
      <div className="gf-form">
        <FormField width={4} placeholder={'required: json path'} onChange={this.onTimeframeChange} label="timeframe" />
        <FormField
          className={'gf-form--grow'}
          labelWidth={4}
          inputWidth={0}
          placeholder={ssql || ''}
          onChange={this.onQueryTextChange}
          label="query"
          tooltip="Chronowave SSQL"
        />
      </div>
    );
  }
}
