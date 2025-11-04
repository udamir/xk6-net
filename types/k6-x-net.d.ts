// TypeScript definitions for xk6-net
// Project: https://github.com/udamir/xk6-net
// Definitions by: Damir Yusipov <https://github.com/udamir>

declare module 'k6/x/net' {
  /**
   * Socket configuration options
   */
  export interface SocketConfig {
    /**
     * Host address to connect to
     */
    host: string;

    /**
     * Port number to connect to
     */
    port: number;

    /**
     * Connection timeout in milliseconds
     * @default 60000
     */
    timeout?: number;

    /**
     * Length of the length field in bytes.
     * If lengthFieldLength is 0, then the length field is not used, and the message length is determined by maxLength.
     * If encoding is "binary" and lengthFieldLength and maxLength are both 0, then message event will not be emitted.
     * @default 0
     */
    lengthFieldLength?: number;

    /**
     * Maximum message length in bytes.
     * If maxLength is 0, then the maximum message length is not limited.
     * @default 0
     */
    maxLength?: number;

    /**
     * Encoding of the message: "utf-8" or "binary".
     * If encoding is "utf-8", then the message is decoded using utf-8 encoding.
     * If encoding is "binary", then the message is decoded using binary encoding.
     * @default "binary"
     */
    encoding?: "utf-8" | "binary";

    /**
     * Delimiter for the message.
     * If encoding is "utf-8" and delimiter is not empty, then it will be used as delimiter for splitting messages.
     * Note that delimiter will be removed from the decoded message.
     * @default ""
     */
    delimiter?: string;

    /**
     * Enable TLS for the connection.
     * If true, the socket will establish a TLS connection using crypto/tls.
     * @default false
     */
    tls?: boolean;

    /**
     * Server name used for SNI and certificate hostname verification.
     * Recommended to set when connecting via IP but verifying a hostname.
     */
    serverName?: string;

    /**
     * Skip certificate verification (NOT recommended for production).
     * Useful for self-signed certificates during testing.
     * @default false
     */
    insecureSkipVerify?: boolean;
  }

  /**
   * Event handler function for raw data events
   */
  export type DataEventHandler = (data: Uint8Array) => void;

  /**
   * Event handler function for decoded message events
   */
  export type MessageEventHandler = (message: Uint8Array | string) => void;

  /**
   * Event handler function for error events
   */
  export type ErrorEventHandler = (error: Error) => void;

  /**
   * Event handler function for connection end events
   */
  export type EndEventHandler = () => void;

  /**
   * Socket event names
   */
  export type SocketEventType = "data" | "message" | "error" | "end";

  /**
   * Socket event handler map
   */
  export interface SocketEventHandlers {
    data: DataEventHandler;
    message: MessageEventHandler;
    error: ErrorEventHandler;
    end: EndEventHandler;
  }

  /**
   * TCP Socket with message handling capabilities for k6 load testing
   */
  export class Socket {
    /**
     * Create a new Socket instance
     */
    constructor();

    /**
     * Establish a TCP connection with the specified configuration
     * @param config Socket configuration including host, port, and connection options
     * @returns Promise that resolves when connection is established
     * @throws Error if connection fails
     */
    connect(config: SocketConfig): void;

    /**
     * Register an event handler for the specified event type
     * @param event Event type to listen for
     * @param handler Event handler function
     */
    on<T extends SocketEventType>(
      event: T,
      handler: SocketEventHandlers[T]
    ): void;

    /**
     * Send a message with length header (if lengthFieldLength > 0)
     * @param data Message data to send
     * @returns Promise that resolves when message is sent
     * @throws Error if socket is not connected or send fails
     */
    send(data: Uint8Array): void;

    /**
     * Send raw data without any headers
     * @param data Raw data to send
     * @returns Promise that resolves when data is sent
     * @throws Error if socket is not connected or write fails
     */
    write(data: Uint8Array): void;

    /**
     * Close the socket connection
     * @returns Promise that resolves when connection is closed
     */
    close(): void;
  }

  /**
   * Net module for creating Socket instances
   */
  export interface NetModule {
    /**
     * Create a new Socket instance with the given configuration
     * @param config Socket configuration options
     * @returns New Socket instance
     */
    Socket: new (config?: SocketConfig) => Socket;
  }

  /**
   * Default export of the net module
   */
  const net: NetModule;
  export default net;
}
