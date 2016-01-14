require('font-awesome-webpack');
require('./main.less');

import React from 'react';
import ReactDOM from 'react-dom';

import Logo from './components/logo';
import ServiceList from './components/service-list';

ReactDOM.render(<Logo />, document.getElementById('header'));
ReactDOM.render(<ServiceList />, document.getElementById('list'));
