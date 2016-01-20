import React from 'react';
import reqwest from 'reqwest';

import SimpleChart from './simple-chart';
import { INTERVAL_SECS } from '../../constants/timer';

// Turn a specification into a Prometheus label match expression
function matchExpr(spec) {
  const labels = [];
  Object.keys(spec).forEach(k => {
    let eq = '=';
    let val = spec[k];
    if (Array.isArray(val)) {
      eq = '=~';
      val = val.join('|');
    }
    labels.push(k + eq + '"' + val + '"');
  });
  return labels.join(',');
}

function statsURL(spec, start, end) {
  const query = 'query=sum(rate(flux_http_total{' + matchExpr(spec) + '}[' + INTERVAL_SECS + 's])) by (code)';
  const interval = '&step=' + INTERVAL_SECS + 's&start=' + start + '&end=' + end;
  return '/stats/api/v1/query_range?' + query + interval;
}

function processValues(val) {
  return {
    date: val[0],
    value: parseFloat(val[1])
  };
}

export default class PrometheusChart extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      okData: [],
      errData: []
    };
    this.chartTimer = null;
    this.getData = this.getData.bind(this);
  }

  getData() {
    const end = +new Date() / 1000;
    const start = end - 900;
    reqwest({
      url: statsURL(this.props.spec, start, end),
      success: json => {
        const result = json.data.result;
        let okData = [];
        let errData = [];
        for (let i = 0; i < result.length; i++) {
          switch (result[i].metric.code) {
          case '200':
            okData = result[i].values.map(processValues);
            break;
          case '302':
            // TODO remove this
            okData = result[i].values.map(processValues);
            break;
          case '500':
            errData = result[i].values.map(processValues);
            break;
          default:
            break;
          }
        }
        this.chartTimer = setTimeout(this.getData, INTERVAL_SECS * 1000);
        if (this.mounted) {
          this.setState({okData, errData});
        }
      }
    });
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
    return <SimpleChart data={this.state.okData} height="100" width="300" />;
  }
}
