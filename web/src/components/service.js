import React from 'react';
import classnames from 'classnames';
import _ from 'lodash';

import Instance from './instance';
import InstanceGroup from './instance-group';
import { GROUP_OPTIONS } from '../constants/options';
import { plural } from '../utils/string-utils';

const NO_GROUPING = 'instance';

export default class Service extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      grouping: NO_GROUPING
    };
  }

  groupInstances(instances, grouping) {
    return _.groupBy(instances, instance => {
      return instance.labels[grouping];
    });
  }

  getCountForGrouping(instances, grouping) {
    if (instances && grouping === NO_GROUPING) {
      return instances.length;
    }
    const grouped = this.groupInstances(instances, grouping);
    return _.size(grouped);
  }

  getInstanceGroups(instances, grouping) {
    return this.groupInstances(instances, grouping);
  }

  renderGroupSelect() {
    const instances = this.props.instances;
    return [NO_GROUPING].concat(GROUP_OPTIONS).map(grouping => {
      const count = this.getCountForGrouping(instances, grouping);
      const className = classnames({
        'service-header-grouping-item': true,
        'service-header-grouping-item-selected': grouping === this.state.grouping
      });
      return (
        <div className={className} key={grouping} onClick={() => this.handleGroupingClick(grouping)}>
          <span className="service-header-grouping-item-count">{count}</span>
          <span className="service-header-grouping-item-label">{plural(grouping, count)}</span>
        </div>
      );
    });
  }

  handleGroupingClick(grouping) {
    const state = {grouping};
    this.setState(state);
  }

  render() {
    const { instances, address, port, protocol, heroMetrics } = this.props;
    const { grouping } = this.state;
    const groupSelect = this.renderGroupSelect();
    const isGrouped = grouping !== NO_GROUPING;
    const instanceGroups = isGrouped ? this.getInstanceGroups(instances, grouping) : [];
    const socket = address ? `${address}:${port}` : '';

    return (
      <div className="service">
        <div className="service-header">
          <div className="service-header-info">
            <div className="service-header-info-title">
              {this.props.name}
            </div>
            <div className="service-header-info-other">
              {socket} {protocol}
            </div>
          </div>
          <div className="service-header-grouping">
            {groupSelect}
          </div>
        </div>
        <div className="service-instances">
          {!isGrouped && instances && _.sortBy(instances, 'name').map(instance =>
            <Instance {...instance}
              key={instance.name} heroMetric={heroMetrics.get(instance.name)}
              />
          )}
          {isGrouped && Object.keys(instanceGroups).map(name =>
            <InstanceGroup group={instanceGroups[name]} key={name} heroMetrics={heroMetrics} />
          )}
        </div>
      </div>
    );
  }
}
