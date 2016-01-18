import React from 'react';

export default class InstanceView extends React.Component {

  renderOtherField(field) {
    const title = `${field.label}: ${field.value}`;
    return (
      <div className="instance-other-field truncate" title={title}>
        <span className="instance-other-field-label">{field.label}:</span>
        <span className="instance-other-field-value">{field.value}</span>
      </div>
    );
  }

  renderOther() {
    const other = [{
      label: 'address',
      value: `${this.props.address}:${this.props.port}`
    }, {
      label: 'rule',
      value: this.props.containerRule
    }, {
      label: 'image',
      value: this.props.labels.image
    }, {
      label: 'tag',
      value: this.props.labels.tag
    }];
    return (
      <div className="instance-other">
        {other.map(field => this.renderOtherField(field))}
      </div>
    );
  }

  render() {
    return (
      <div className="instance">
        <div className="instance-metric">
          <span className="instance-metric-value">{Number(Math.random()).toFixed(2)}</span>
          <span className="instance-metric-unit">QPS</span>
        </div>
        <div className="instance-title truncate" title={this.props.name}>
          {this.props.name}
        </div>
        {this.renderOther()}
      </div>
    );
  }
}
