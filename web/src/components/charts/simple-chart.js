import React from 'react';
import ReactDOM from 'react-dom';
import d3 from 'd3';
import debug from 'debug';

const log = debug('flux:simple-chart');

const customTimeFormat = d3.time.format.multi([
  ['.%L', function(d) { return d.getMilliseconds(); }],
  [':%S', function(d) { return d.getSeconds(); }],
  ['%I:%M', function(d) { return d.getMinutes(); }],
  ['%H:%M', function(d) { return d.getHours(); }],
  ['%a %d', function(d) { return d.getDay() && d.getDate() !== 1; }],
  ['%b %d', function(d) { return d.getDate() !== 1; }],
  ['%B', function(d) { return d.getMonth(); }],
  ['%Y', function() { return true; }]
]);

export default class SimpleChart extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.state = {
      selectedTime: null,
    };
  }

  componentDidUpdate() {
    this.renderAxis();
  }

  componentDidMount() {
    this.renderAxis();
  }

  renderAxis() {
    const xAxisNode = ReactDOM.findDOMNode(this.xAxisRef);
    d3.select(xAxisNode).call(this.xAxis);

    const yAxisNode = ReactDOM.findDOMNode(this.yAxisRef);
    d3.select(yAxisNode).call(this.yAxis);
  }

  render() {
    const {width: outerWidth, height: outerHeight, data} = this.props;
    const margins = {top: 6, bottom: 16, left: 64, right: 16};
    const w = outerWidth - margins.left - margins.right;
    const h = outerHeight - margins.top - margins.bottom;

    const x = d3.time.scale()
      .range([0, w])
      .domain(d3.extent(data, d => d.date));

    this.xScale = x;

    this.xAxis = d3.svg.axis()
      .ticks(4)
      .scale(x)
      .tickSize(-h)
      .tickFormat(customTimeFormat);

    const y = d3.scale.linear()
      .range([h, 0])
      .domain([0, d3.max(data, d => d.value)]);

    this.yAxis = d3.svg.axis()
      .ticks(5)
      .scale(y)
      .orient('left')
      .tickSize(-w);

    const line = d3.svg.line()
      .interpolate('monotone')
      .x(d => x(d.date))
      .y(d => y(d.value));

    const style = {width: outerWidth, height: outerHeight};
    const selectedX = x(this.state.selectedTime);
    log('sel', selectedX, this.state.selectedTime);

    const voronoi = d3.geom.voronoi()
      .x(d => x(d.date))
      .y(0)
      .clipExtent([[0, 0], [w, h]]);

    const shapes = voronoi(data);
    const toLine = (d) => ('M' + d.join('L') + 'Z');

    return (
      <div style={style}
        className="chart">
        <svg style={style}>
          <g transform={`translate(${margins.left}, ${margins.top})`}>
            <rect className="background" width={w} height={h} />
            <g className="x axis" transform={`translate(0, ${h + 4})`}
              ref={(ref) => this.xAxisRef = ref}></g>
            <g className="y axis" transform={`translate(-4, 0)`}
              ref={(ref) => this.yAxisRef = ref}></g>
            <path d={line(data)} />
            {shapes.map((s, i) => <path className="voro" key={i} d={toLine(s)} />)}
          </g>
        </svg>
      </div>
    );
  }
}
