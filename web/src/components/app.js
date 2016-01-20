import React from 'react';
import reqwest from 'reqwest';
import { OrderedSet } from 'immutable';

import ServiceList from './service-list';
import ServiceNavigation from './service-navigation';
import Logo from './logo';

const makeOrderedSet = OrderedSet;

export default class App extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      filteredServices: makeOrderedSet(),
      services: []
    };
    this.refreshData = this.refreshData.bind(this);
    this.toggleFilteredService = this.toggleFilteredService.bind(this);
  }

  refreshData() {
    reqwest({
      url: '/api/services',
      type: 'json',
      success: services => {
        this.setState({services: services});
        setTimeout(this.refreshData, 10000);
      }
    });
  }

  componentDidMount() {
    this.refreshData();
  }

  toggleFilteredService(name) {
    let { filteredServices } = this.state;
    if (filteredServices.has(name)) {
      filteredServices = filteredServices.delete(name);
    } else {
      filteredServices = filteredServices.add(name);
    }
    this.setState({filteredServices});
  }

  render() {
    const { filteredServices } = this.state;
    const serviceFilter = service => (filteredServices.size === 0 || filteredServices.has(service.name));

    return (
      <div className="app">
        <header className="app-header">
          <Logo />
          <ServiceNavigation services={this.state.services} filteredServices={filteredServices}
           handleServiceClick={this.toggleFilteredService} />
        </header>
        <div className="app-main">
          <ServiceList services={this.state.services.filter(serviceFilter)} />
        </div>
      </div>
    );
  }
}
