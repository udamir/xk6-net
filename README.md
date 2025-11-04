# xk6-net

A k6 extension for TCP socket communication with message handling capabilities

## Build

To build a `k6` binary with this plugin, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:

```sh
go install go.k6.io/xk6/cmd/xk6@latest
```

2. Build the binary:

```sh
xk6 build --with github.com/udamir/xk6-net@latest
```

## Example

```javascript
import net from 'k6/x/net';
import { check } from 'k6';

const socket = new net.Socket();

export default function () {

  socket.connect('localhost', 5000, {
    timeout: 5000,
    /**
     * Length of the length field in bytes
     * if lengthFieldLength is 0, then the length field is not used, and the message length is determined by maxLength. 
     * If encoding is "binary" and lengthFieldLength and maxLength are both 0, then message event will not be emitted.
     */
    lengthFieldLength: 4,
    /**
     * Maximum message length in bytes
     * If maxLength is 0, then the maximum message length is not limited
     */
    maxLength: 1 * 1024 * 1024,
    /**
     * Encoding of the message: "utf-8" or "binary"
     * If encoding is "utf-8", then the message is decoded using utf-8 encoding
     * If encoding is "binary", then the message is decoded using binary encoding
     */
    encoding: "binary",
    /**
     * Delimiter for the message
     * If encoding is "utf-8" and delimiter is not empty, then it will be used as delimiter for splitting messages
     * Note that delimiter will be removed from the decoded message
     */
    delimiter: "",
    /**
     * Enable TLS for the connection
     */
    tls: false,
    /**
     * Server name for SNI and certificate verification (optional)
     */
    serverName: "",
    /**
     * Skip certificate verification - NOT recommended for production
     */
    insecureSkipVerify: false,
  })

  socket.on("data", (data) => {
    // handle raw data (Uint8Array)
    console.log("Received raw data:", data.length, "bytes");
  })

  socket.on("message", (message) => {
    // handle unpacked message (decoded based on encoding)
    console.log("Received message:", message);
  })

  socket.on("error", (error) => {
    // handle errors
    console.error("Socket error:", error);
  })

  socket.on("end", () => {
    // handle socket termination
    console.log("Socket connection ended");
  })

  // send message with length header (4 bytes big-endian)
  const messageData = new Uint8Array([1, 2, 3, 4, 5]);
  socket.send(messageData);

  // send raw data without length header
  socket.write(messageData);

  // close socket
  socket.close();
}
```

## TypeScript Support

xk6-net includes comprehensive TypeScript type definitions for enhanced development experience.

### Installation

1. Install k6 types for your project:

```bash
npm install --save-dev @types/k6
```

2. Copy the type definitions from this repository to your project:

```bash
# Create a types directory if it doesn't exist
mkdir -p types

# Copy the k6-x-net.d.ts file to your types directory
cp path/to/xk6-net/types/k6-x-net.d.ts types/
```

3. Ensure your `tsconfig.json` includes the types directory:

```json
{
  "compilerOptions": {
    "typeRoots": ["./node_modules/@types", "./types"]
  }
}
```

### Type Definitions

The module exports the following TypeScript interfaces:

- `SocketConfig` - Configuration options for Socket.connect()
- `Socket` - Main socket class with all methods
- `DataEventHandler` - Type for "data" event handlers
- `MessageEventHandler` - Type for "message" event handlers  
- `ErrorEventHandler` - Type for "error" event handlers
- `EndEventHandler` - Type for "end" event handlers

### Usage with TypeScript

```typescript
import net from 'k6/x/net';

const socket = new net.Socket();

// TypeScript will provide autocomplete and type checking
socket.connect('localhost', 5000, {
  timeout: 5000,
  lengthFieldLength: 4,
  encoding: 'binary'
});
```

# License

MIT
