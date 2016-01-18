import React from 'react';

export default class ServiceNavigationItem extends React.Component {
  render() {
    const title = `${this.props.name}, `
      + `${this.props.address}:${this.props.port}, `
      + `${this.props.instances.length} instances`;
    return (
      <div className="service-navigation-item" title={title}>
        {this.props.name} ({this.props.instances.length})
      </div>
    );
  }
}
