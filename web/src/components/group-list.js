import React from 'react';

import GroupView from './group-view';

// Return an object with just the fields given
function project(object, fields) {
  const result = {};
  fields.forEach(function(f) {
    result[f] = object[f];
  });
  return result;
}

// Group the instances given by the values of the labels given,
// e.g., by tag.
function groupInstancesBy(instances, labels) {
  const result = {};
  instances.forEach(function(instance) {
    const key = JSON.stringify(project(instance.labels, labels));
    if (result[key] === undefined) {
      result[key] = [instance];
    } else {
      result[key].push(instance);
    }
  });
  return result;
}

export default class GroupList extends React.Component {
  render() {
    const groups = groupInstancesBy(this.props.instances, this.props.fields);
    const items = [];
    Object.keys(groups).forEach(g => {
      items.push(<li key={g}><GroupView name={g} instances={groups[g]}/></li>);
    });
    return (<ul>{items}</ul>);
  }
}
