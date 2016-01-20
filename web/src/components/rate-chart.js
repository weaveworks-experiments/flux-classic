import React from 'react';
import reqwest from 'reqwest';

import { INTERVAL_SECS } from '../constants/timer';

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

function zip(as, bs, fn) {
  const len = Math.min(as.length, bs.length);
  const res = new Array(len);
  for (let i = 0; i < len; i++) {
    res[i] = fn(as[i], bs[i]);
  }
  return res;
}

export default class RateChart extends React.Component {
  getData(start, end, k) {
    reqwest({
      url: statsURL(this.props.spec, start, end),
      success: function(json) {
        const result = json.data.result;
        let okData = [];
        let errData = [];
        for (let i = 0; i < result.length; i++) {
          switch (result[i].metric.code) {
          case '200':
            okData = result[i].values;
            break;
          case '500':
            errData = result[i].values;
            break;
          default:
            break;
          }
        }
        k(okData, errData);
      }
    });
  }

  stepChart(chart, lastNow) {
    const now = +new Date() / 1000;
    this.getData(lastNow, now, function(okData, errData) {
      if (this.mounted) {
        let nextNow = lastNow;
        const data = zip(okData, errData, function(ok, err) {
          return [{OK: Number(ok[1]), Error: Number(err[1])}, ok[0]];
        });
        data.forEach(function(datum) {
          if (datum[1] > lastNow) {
            chart.series.addData.apply(chart.series, datum);
            nextNow = datum[1];
          }
        });
        this.renderChart(chart);
        setTimeout(this.stepChart.bind(this, chart, nextNow), INTERVAL_SECS * 1000);
      }
    }.bind(this));
  }

  componentDidMount() {
    const end = +new Date() / 1000;
    const start = end - 300;
    this.mounted = true;
    this.getData(start, end, function(okData, errData) {
      console.log(start, end, okData, errData);
    });
  }

  render() {
    return (<div>
              <div className="yAxis" ref="chartY"/>
              <div className="chart" ref="chart"/>
              <div className="legend" ref="legend"/>
            </div>);
  }
}
