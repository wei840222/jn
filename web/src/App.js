import React from "react";
import CodeEditor from '@uiw/react-textarea-code-editor';
import { Layout, Space, Card, Button } from '@arco-design/web-react';
import JSONFormatter from 'json-formatter-js'
const Content = Layout.Content;

class App extends React.Component {
  constructor(props) {
    super(props);
    this.handleRun = this.handleRun.bind(this);
    this.handleChange = this.handleChange.bind(this);
    this.state = {
      script: 'const result = data;\n[result].map(item => item.a);',
      data: JSON.stringify({ a: "cccc" }, null, 2),
    }
    this.resultFormatterReference = React.createRef();
    this.resultFormatter = new JSONFormatter('Click Run button to run script...', Infinity, {
      hoverPreviewEnabled: true,
      hoverPreviewArrayCount: 100,
      hoverPreviewFieldCount: 5,
      theme: '',
      animateOpen: true,
      animateClose: true,
      useToJSON: true,
      maxArrayItems: 100,
      exposePath: false
    });
  }

  componentDidMount() {
    const script = localStorage.getItem('script')
    if (script) {
      this.setState({ script });
    }
    const data = localStorage.getItem('data')
    if (data) {
      this.setState({ data });
    }
    this.resultFormatterReference.current.innerHTML = '';
    this.resultFormatterReference.current.appendChild(this.resultFormatter.render());
  }

  handleChange(event) {
    localStorage.setItem(event.target.id, event.target.value);
    this.setState({ [event.target.id]: event.target.value });
  }

  async handleRun() {
    let data
    try {
      data = JSON.parse(this.state.data);
    } catch {
      data = this.state.data;
    }
    const response = await fetch('/', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        script: this.state.script,
        data
      })
    });
    const body = await response.json();
    this.resultFormatter.json = body
    this.resultFormatterReference.current.innerHTML = '';
    this.resultFormatterReference.current.appendChild(this.resultFormatter.render());
  }

  render() {
    return (
      <Content style={{ display: 'flex', justifyContent: 'center' }}>
        <Card
          title='jsrun'
          style={{ marginTop: '5%', width: 1550 }}
          extra={
            <Button onClick={this.handleRun}>
              Run
            </Button>
          }
        >
          <Space align={'start'}>
            <CodeEditor id="data"
              value={this.state.data}
              language="json"
              placeholder="Please enter JSON."
              onChange={this.handleChange}
              padding={15}
              style={{
                height: 600, width: 500,
                fontSize: 16,
                backgroundColor: "#f5f5f5",
                fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Consolas,Liberation Mono,Menlo,monospace',
              }}
            />
            <CodeEditor id="script"
              value={this.state.script}
              language="javascript"
              placeholder="Please enter JS code."
              onChange={this.handleChange}
              padding={15}
              style={{
                height: 600, width: 500,
                fontSize: 16,
                backgroundColor: "#f5f5f5",
                fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Consolas,Liberation Mono,Menlo,monospace',
              }}
            />
            <div ref={this.resultFormatterReference} style={{
              height: 570, width: 470,
              padding: '15px',
              fontSize: 16,
              backgroundColor: "#f5f5f5",
              fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Consolas,Liberation Mono,Menlo,monospace',
            }}></div>
          </Space>
        </Card>
      </Content>
    )
  }
}

export default App;
