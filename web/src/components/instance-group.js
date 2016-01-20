import React from 'react';

import { plural } from '../utils/string-utils';

function rnd() {
  return Number(Math.random()).toFixed(2);
}

export default class InstanceGroup extends React.Component {
  render() {
    const count = this.props.group.length;
    const instance = this.props.group[0];
    const name = `${instance.labels.image}:${instance.labels.tag}`;
    const values = this.props.group.map(rnd);

    return (
      <div className="instance">
        <div className="instance-metric">
          {/* this should be the group avg */}
          <span className="instance-metric-value">{values[0]}</span>
          <span className="instance-metric-unit">QPS</span>
          <div className="instance-metric-aggregate-wrap">
            <div className="instance-metric-aggregate">
              <span className="instance-metric-aggregate-icon fa fa-sort-up" />
              <span className="instance-metric-aggregate-value">{Math.max(...values)}</span>
            </div>
            <div className="instance-metric-aggregate">
              <span className="instance-metric-aggregate-icon fa fa-sort-down" />
              <span className="instance-metric-aggregate-value">{Math.min(...values)}</span>
            </div>
          </div>
        </div>
        <div className="instance-title truncate" title={'Name: ' + name}>
          {name}
        </div>
        <div className="instance-other">
          <div className="instance-other-field">
            {count}
            {' '}
            <span className="instance-other-field-label">
              {plural('instance', count)}
            </span>
          </div>
        </div>
      </div>
    );
  }
}
