import React from 'react';

import RateChart from './rate-chart';
import { labels, maybeTruncate } from '../utils/string-utils';

export default class InstanceView extends React.Component {
  render() {
    return (<div className="instanceView">
              <div>{this.props.instance.address}:{this.props.instance.port}
                ({maybeTruncate(this.props.instance.name)};
                 {labels(this.props.instance.labels)})
              </div>
              <RateChart spec={{individual: this.props.instance.name}}/>
            </div>);
  }
}
