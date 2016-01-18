import React from 'react';
import classnames from 'classnames';
import _ from 'lodash';

import InstanceList from './instance-list';
import { GROUP_OPTIONS } from '../constants/options';
import { plural } from '../utils/string-utils';

export default class ServiceView extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      grouping: 'instance'
    };
  }

  getCountForGrouping(grouping) {
    const instances = this.props.instances;
    if (grouping === 'instance') {
      return instances.length;
    }
    const grouped = _.groupBy(instances, grouping);
    return _.size(grouped);
  }

  renderGroupSelect() {
    return ['instance'].concat(GROUP_OPTIONS).map(grouping => {
      const count = this.getCountForGrouping(grouping);
      const className = classnames({
        'service-header-grouping-item': true,
        'service-header-grouping-item-selected': grouping === this.state.grouping
      });
      return (
        <div className={className}>
          <span className="service-header-grouping-item-count">{count}</span>
          <span className="service-header-grouping-item-label">{plural(grouping, count)}</span>
        </div>
      );
    });
  }

  render() {
    const groupSelect = this.renderGroupSelect();

    return (
      <div className="service">
        <div className="service-header">
          <div className="service-header-info">
            <div className="service-header-info-title">
              {this.props.name}
            </div>
            <div className="service-header-info-other">
              {this.props.address}:{this.props.port} {this.props.protocol}
            </div>
          </div>
          <div className="service-header-grouping">
            {groupSelect}
          </div>
        </div>
        <div className="service-instances">
          <InstanceList instances={this.props.instances} />
        </div>
      </div>
    );
  }
}
