import React from 'react';
import reqwest from 'reqwest';

import ServiceList from './service-list';
import ServiceNavigation from './service-navigation';
import Logo from './logo';

export default class App extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      services: []
    };
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
    return (
      <div className="app">
        <header className="app-header">
          <Logo />
          <ServiceNavigation services={this.state.services} />
        </header>
        <div className="app-main">
          <ServiceList services={this.state.services} />
        </div>
      </div>
    );
  }
}
