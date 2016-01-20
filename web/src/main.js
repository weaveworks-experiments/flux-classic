require('font-awesome-webpack');
require('./main.less');

import React from 'react';
import ReactDOM from 'react-dom';

import App from './components/app';

ReactDOM.render(<App />, document.getElementById('app'));
