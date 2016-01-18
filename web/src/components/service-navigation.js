import React from 'react';

import ServiceNavigationItem from './service-navigation-item';
import { plural } from '../utils/string-utils';

export default class ServiceNavigation extends React.Component {

  render() {
    const {services, filteredServices, handleServiceClick } = this.props;
    const serviceItems = services.map(service => <ServiceNavigationItem
      {...service} key={service.name} active={filteredServices.has(service.name)}
      handleClick={() => handleServiceClick(service.name)} />);
    return (
      <div className="service-navigation">
        <div className="service-navigation-count">
          <span className="service-navigation-count-value">
            {services.length}
          </span>
          <span className="service-navigation-count-label">
            {plural('service', services.length)}
          </span>
        </div>
        <div className="service-navigation-items">
          {serviceItems}
        </div>
      </div>
    );
  }
}
