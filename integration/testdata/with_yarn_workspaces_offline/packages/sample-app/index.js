const express = require('express');
const app = express();
const config = require('@sample/sample-config');

app.get('/', (req, res) => {
    res.send({
        config: config(),
    });
});

const port = process.env.PORT || 8080;

app.listen(port, () => console.log(`Sample app listening on port ${ port }!`));