import React from 'react';
import classnames from 'classnames';

import { formatMetric } from '../utils/string-utils';
import { QUERY_WINDOW_SECS } from '../constants/timer';

export default class Instance extends React.Component {

  renderOther() {
    const imageTitle = `Image:tag: ${this.props.labels.image}:${this.props.labels.tag}`;
    const address = (this.props.state === 'live') ? `${this.props.address}:${this.props.port}` : this.props.state;
    return (
      <div className="instance-other">
        <div className="instance-other-field truncate" title={imageTitle}>
          {this.props.labels.image}:{this.props.labels.tag}
        </div>
        <div className="instance-other-field truncate" title={'Address: ' + address}>
          {address}
          {' ('}
          <span className="instance-other-field-label">host:</span>
          {this.props.host}
          {')'}
        </div>
      </div>
    );
  }

  render() {
    const heroMetric = this.props.heroMetric === undefined ? '\u2014' : formatMetric(this.props.heroMetric);
    const className = classnames({
      instance: true,
      'instance-selected': this.props.selected,
      'state-live': this.props.state === 'live',
      'state-unused': this.props.state !== 'live',
    });
    return (
      <div className={className} key={this.props.name} onClick={this.props.handleClick}>
        <div className="instance-metric">
          <span className="instance-metric-value">{heroMetric}</span>
            <span className="instance-metric-unit">QPS (last {QUERY_WINDOW_SECS}s)</span>
        </div>
        <div className="instance-title truncate" title={'Name: ' + this.props.name}>
          {this.props.name}
        </div>
        {this.renderOther()}
      </div>
    );
  }
}