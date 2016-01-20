import React from 'react';
import SimpleChart from '../charts/simple-chart';
import d3 from 'd3';

const simpleData = [
  1, 3, 2, 5, 7, 3, 1, 2, 6, 8, 7
];

function generateDates(interval, n) {
  const start = new Date('2016-01-10');
  const end = interval.offset(start, n);
  return interval.range(start, end);
}

function zipValsDates(values, dates) {
  return values.map((v, i) => {
    return {value: v, date: dates[i]};
  });
}

const intervals = [
  d3.time.minute,
  d3.time.hour,
  d3.time.day
];

export default class ComponentExamples extends React.Component {
  render() {
    return (
      <div className="app-main">
        <h1>Charts</h1>
        <h2>Various time intervals</h2>
        {intervals.map((interval, i) => {
          const times = generateDates(interval, simpleData.length);
          const data = zipValsDates(simpleData, times);
          return (
            <SimpleChart key={i} width={500} height={200} data={data} />
          );
        })}
        <h2>Various value ranges</h2>
        {[10, 10000, 1000000].map((n, i) => {
          const times = generateDates(d3.time.hour, simpleData.length);
          const data = zipValsDates(simpleData.map(v => v * n), times);
          return (
            <SimpleChart key={i} width={500} height={200} data={data} />
          );
        })}
        <h2>Updating data</h2>
      </div>
    );
  }
}
