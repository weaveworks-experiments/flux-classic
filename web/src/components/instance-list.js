import React from 'react';

import Instance from './instance';

export default class InstanceList extends React.Component {
  render() {
    return (
      <div className="instance-list">
        {this.props.instances.map(instance => <Instance {...instance} />)}
      </div>
    );
  }
}
