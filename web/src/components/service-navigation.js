import React from 'react';

import ServiceNavigationItem from './service-navigation-item';

export default class ServicesList extends React.Component {

  render() {
    const serviceItems = this.props.services.map(service => <ServiceNavigationItem {...service} />);
    return (
      <div className="service-navigation">
        <div className="service-navigation-count">
          <span className="service-navigation-count-value">
            {this.props.services.length}
          </span>
          <span className="service-navigation-count-label">
            services
          </span>
        </div>
        <div className="service-navigation-items">
          {serviceItems}
        </div>
      </div>
    );
  }
}
