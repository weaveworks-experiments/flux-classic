import React from 'react';
import classnames from 'classnames';

import { formatMetric } from '../utils/string-utils';
import { QUERY_WINDOW_SECS } from '../constants/timer';

export default class Instance extends React.Component {

  renderOther() {
    const imageTitle = `Image:tag: ${this.props.labels.image}:${this.props.labels.tag}`;
    const addressTitle = `Address: ${this.props.address}:${this.props.port}`;
    return (
      <div className="instance-other">
        <div className="instance-other-field truncate" title={imageTitle}>
          {this.props.labels.image}:{this.props.labels.tag}
        </div>
        <div className="instance-other-field truncate" title={addressTitle}>
          {`${this.props.address}:${this.props.port}`}
          {' ('}
          <span className="instance-other-field-label">host:</span>
          {this.props.ownerID}
          {')'}
        </div>
      </div>
    );
  }

  render() {
    const heroMetric = this.props.heroMetric === undefined ? '--' : formatMetric(this.props.heroMetric);
    const className = classnames({
      instance: true,
      'instance-selected': this.props.selected
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
