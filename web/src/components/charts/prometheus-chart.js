import React from 'react';
import ReactDOM from 'react-dom';
import d3 from 'd3';

import SimpleChart from './simple-chart';
import { INTERVAL_SECS } from '../../constants/timer';
import { now, requestRange, processValues } from '../../utils/prometheus-utils';

function toSeriesSet(series) {
  const color = d3.scale.category20();
  return series.map((s, i) => {
    return {
      id: 'ewq' + i,
      color: color(i),
      data: s
    };
  });
}

export default class PrometheusChart extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      okData: [],
      errData: [],
      width: 0
    };
    this.chartTimer = null;
    this.getData = this.getData.bind(this);
    this.receiveData = this.receiveData.bind(this);
  }

  receiveData(json) {
    const result = json.data.result;
    let okData = [];
    let errData = [];
    for (let i = 0; i < result.length; i++) {
      switch (result[i].metric.code) {
      case '200':
        okData = result[i].values.map(processValues);
        break;
      case '500':
        errData = result[i].values.map(processValues);
        break;
      default:
        break;
      }
    }
    // sync chart request to other requests by clocking into a slot
    const timeToNextSlot = (now() + INTERVAL_SECS) * 1000 - new Date;
    this.chartTimer = setTimeout(this.getData, timeToNextSlot);
    if (this.mounted) {
      const container = ReactDOM.findDOMNode(this).parentElement;
      const width = container.clientWidth || 100;
      this.setState({okData, errData, width});
    }
  }

  getData() {
    const end = now();
    const start = end - 300;
    requestRange(this.props.spec, start, end, this.receiveData);
  }

  componentDidMount() {
    this.mounted = true;
    this.getData();
  }

  componentWillUnmount() {
    this.mounted = false;
    clearTimeout(this.chartTimer);
  }

  render() {
    const data = toSeriesSet([this.state.okData, this.state.errData]);
    return <SimpleChart data={data} label={this.props.label} height="100" width={this.state.width} />;
  }
}
