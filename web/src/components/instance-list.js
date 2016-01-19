import React from 'react';
import reqwest from 'reqwest';

import InstanceView from './instance-view';
import { INTERVAL_SECS } from '../constants/timer';

export default class InstanceList extends React.Component {
  constructor(props, context) {
    super(props, context);
    this.state = {data: []};
    this.receiveData = this.receiveData.bind(this);
    this.refreshData = this.refreshData.bind(this);
  }

  refreshData() {
    reqwest({
      url: '/api/' + this.props.service + '/',
      type: 'json',
      success: this.receiveData
    });
  }

  receiveData(data) {
    this.setState({data: data.children});
    setTimeout(this.refreshData, INTERVAL_SECS * 1000);
  }

  componentDidMount() {
    this.refreshData();
  }

  render() {
    const instanceNodes = this.state.data.map(function(i) {
      return (<li key={i.name}><InstanceView instance={i}/></li>);
    });
    return (<ul>
            {instanceNodes}
            </ul>);
  }
}
