import React from 'react';
import classnames from 'classnames';

import { formatMetric, plural } from '../utils/string-utils';

export default class InstanceGroup extends React.Component {
  render() {
    const { group, heroMetrics } = this.props;
    const count = group.length;
    const instance = group[0];
    const name = `${instance.labels.image}:${instance.labels.tag}`;
    const metrics = group.map(ins => heroMetrics.get(ins.name)).filter(val => val !== undefined);
    const avg = metrics.length ? formatMetric(metrics.reduce((res, val) => res + val) / metrics.length) : 'n/a';
    const max = metrics.length ? formatMetric(Math.max(...metrics)) : '-';
    const min = metrics.length ? formatMetric(Math.min(...metrics)) : '-';
    const className = classnames({
      instance: true,
      'instance-selected': this.props.selected
    });

    return (
      <div className={className} onClick={this.props.handleClick}>
        <div className="instance-metric">
          {/* this should be the group avg */}
          <span className="instance-metric-value">{avg}</span>
          <span className="instance-metric-unit">QPS</span>
          <div className="instance-metric-aggregate-wrap">
            <div className="instance-metric-aggregate">
              <span className="instance-metric-aggregate-icon fa fa-sort-up" />
              <span className="instance-metric-aggregate-value">{max}</span>
            </div>
            <div className="instance-metric-aggregate">
              <span className="instance-metric-aggregate-icon fa fa-sort-down" />
              <span className="instance-metric-aggregate-value">{min}</span>
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
