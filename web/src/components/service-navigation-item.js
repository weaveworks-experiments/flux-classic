import React from 'react';
import classnames from 'classnames';

import { plural } from '../utils/string-utils';

export default class ServiceNavigationItem extends React.Component {
  render() {
    const title = [this.props.name];
    const text = [this.props.name];
    const count = this.props.instances ? this.props.instances.length : 0;
    if (this.props.address) {
      title.push(`${this.props.address}:${this.props.port}`);
    }
    title.push(`${count} ${plural('instance', count)}`);
    text.push(`(${count})`);

    const className = classnames({
      'service-navigation-item': true,
      'service-navigation-item-active': this.props.active
    });
    return (
      <div className={className} title={title.join(', ')} onClick={this.props.handleClick}>
        {text.join(' ')}
      </div>
    );
  }
}
