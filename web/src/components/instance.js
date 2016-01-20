import React from 'react';

import PrometheusChart from './charts/prometheus-chart';

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
    return (
      <div className="instance" key={this.props.name}>
        <div className="instance-metric">
          <span className="instance-metric-value">{Number(Math.random()).toFixed(2)}</span>
          <span className="instance-metric-unit">QPS</span>
        </div>
        <div className="instance-title truncate" title={'Name: ' + this.props.name}>
          {this.props.name}
        </div>
        {this.renderOther()}
        <PrometheusChart spec={{individual: [this.props.name]}}/>
      </div>
    );
  }
}
