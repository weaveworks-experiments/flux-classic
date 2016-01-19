import React from 'react';

export default class GroupingSelect extends React.Component {
  constructor(props, context) {
    super(props, context);
    this.state = {selected: []};
    this.changeSelect = this.changeSelect.bind(this);
  }

  render() {
    const self = this;
    const selected = this.state.selected.map(function(s) {
      return (<span className="selected">{s}
                <span
                  className="selected-x"
                  onClick={self.clickX.bind(self, s)}>x</span>
              </span>);
    });
    const options = this.props.available.map(function(f) {
      return (<option value={f}>{f}</option>);
    });
    return (<div>
              {selected}
              Add:
              <select defaultValue="" onChange={this.changeSelect}>
                <option value="">(select field)</option>
                {options}
              </select>
            </div>);
  }

  clickX(opt) {
    const fields = [];
    this.state.selected.forEach(i => {
      const s = this.state.selected[i];
      if (s !== opt) fields.push(s);
    });
    this.setState({selected: fields});
  }

  changeSelect(ev) {
    const opt = ev.target.value;
    if (opt === '') return;
    const fields = [];
    this.state.selected.forEach(i => {
      if (this.state.selected[i] === opt) return;
      fields.push(this.state.selected[i]);
    });
    fields.push(opt);
    this.setState({selected: fields});
    this.props.callback(fields);
  }
}
