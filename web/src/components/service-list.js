import React from 'react';

import Service from './service';

export default class ServicesList extends React.Component {

  render() {
    const services = this.props.services.map(service =>
      <Service {...service} heroMetrics={this.props.heroMetrics} key={service.name} />
    );
    return (
      <div className="service-list">
        {services}
      </div>
    );
  }
}
