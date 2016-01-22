import React from 'react';
import reqwest from 'reqwest';
import { Map, OrderedSet } from 'immutable';
import _ from 'lodash';

import ServiceList from './service-list';
import ServiceNavigation from './service-navigation';
import Logo from './logo';
import { requestLastValues } from '../utils/prometheus-utils';

const makeOrderedSet = OrderedSet;
const makeMap = Map;

export default class App extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      filteredServices: makeOrderedSet(),
      heroMetrics: makeMap(),
      services: []
    };
    this.refreshServices = this.refreshServices.bind(this);
    this.refreshMetrics = this.refreshMetrics.bind(this);
    this.receiveMetrics = this.receiveMetrics.bind(this);
    this.toggleFilteredService = this.toggleFilteredService.bind(this);
  }

  refreshServices() {
    reqwest({
      url: '/api/services',
      type: 'json',
      success: services => {
        this.refreshMetrics(services);
        this.setState({services: _.sortBy(services, 'name')});
        setTimeout(this.refreshServices, 10000);
      }
    });
  }

  refreshMetrics(services) {
    if (services.length > 0) {
      // getting all instance names, maybe we should just allow them all (no filter)
      const instances = services.reduce((arr, service) => {
        return arr.concat(service.instances ? service.instances.map(instance => instance.name) : []);
      }, []);
      requestLastValues(instances, this.receiveMetrics);
    }
  }

  receiveMetrics(values) {
    this.setState({heroMetrics: makeMap(values)});
  }

  componentDidMount() {
    this.refreshServices();
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
          <ServiceList services={this.state.services.filter(serviceFilter)}
            heroMetrics={this.state.heroMetrics} />
        </div>
      </div>
    );
  }
}
