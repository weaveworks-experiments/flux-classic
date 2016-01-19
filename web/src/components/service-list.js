import React from 'react';
import reqwest from 'reqwest';

import ServiceView from './service-view';

export default class InstanceList extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {services: []};
    this.refreshData = this.refreshData.bind(this);
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

  render() {
    const serviceNodes = this.state.services.map(function(service) {
      return (<li key={service.name} className="">
              <ServiceView service={service}/>
              </li>);
    });
    return (
        <ul className="serviceList">
        {serviceNodes}
      </ul>
    );
  }
}
