import React from 'react';
import reqwest from 'reqwest';

import GroupList from './group-list';
import GroupingSelect from './grouping-select';
import { GROUP_OPTIONS } from '../constants/options';

export default class ServiceView extends React.Component {
  constructor(props, context) {
    super(props, context);
    this.state = {fields: [], instances: []};
    this.setFields = this.setFields.bind(this);
  }

  setFields(fields) {
    this.setState({fields: fields});
  }

  refreshData() {
    reqwest({
      url: '/api/' + this.props.service.name + '/',
      type: 'json',
      success: service => {
        this.setState({instances: service.children});
        setTimeout(this.refreshData, 10000);
      }
    });
  }

  componentDidMount() {
    this.refreshData();
  }

  render() {
    const list = (<GroupList instances={this.state.instances} fields={this.state.fields}/>);
    return (
        <div className="serviceView">
          <div>
            <strong>
              {this.props.service.name}
              {this.props.service.address}:{this.props.service.port}
              ({this.state.instances.length} instances)
            </strong>
            <div><GroupingSelect available={GROUP_OPTIONS} callback={this.setFields}/></div>
          </div>
          {list}
        </div>
    );
  }
}
