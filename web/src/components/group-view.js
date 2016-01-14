import React from 'react';

import RateChart from './rate-chart';
import { labels } from '../utils/string-utils';

function instanceName(instance) {
  return instance.name;
}

export default class GroupView extends React.Component {
  render() {
    return (
        <div className="groupView">
          <strong>{labels(JSON.parse(this.props.name))} ({this.props.instances.length} instances)</strong>
          <RateChart spec={{individual: this.props.instances.map(instanceName)}}/>
        </div>);
  }
}
