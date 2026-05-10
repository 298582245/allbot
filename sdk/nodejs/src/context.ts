/**
 * Context 类 - 提供插件开发的统一 API
 */

export interface ContextOptions {
  platform: string;
  userId: string;
  groupId: string;
  content: string;
  messageId: string;
  pluginId: string;
  grpcChannel: any;
}

export class Context {
  public platform: string;
  public userId: string;
  public groupId: string;
  public content: string;
  public messageId: string;
  public pluginId: string;
  private grpcChannel: any;

  constructor(options: ContextOptions) {
    this.platform = options.platform;
    this.userId = options.userId;
    this.groupId = options.groupId;
    this.content = options.content;
    this.messageId = options.messageId;
    this.pluginId = options.pluginId;
    this.grpcChannel = options.grpcChannel;
  }

  /**
   * 回复消息
   */
  async reply(text: string): Promise<boolean> {
    // TODO: 调用 gRPC ReplyRequest
    console.log(`[Reply] ${text}`);
    return true;
  }

  /**
   * 发送图片
   */
  async sendImage(imageUrl: string): Promise<boolean> {
    // TODO: 调用 gRPC SendImageRequest
    console.log(`[SendImage] ${imageUrl}`);
    return true;
  }

  /**
   * 发送文件
   */
  async sendFile(filePath: string): Promise<boolean> {
    // TODO: 调用 gRPC SendFileRequest
    console.log(`[SendFile] ${filePath}`);
    return true;
  }

  /**
   * 等待用户的下一条消息（连续对话）
   */
  async listen(timeout: number = 60): Promise<string> {
    // TODO: 调用 gRPC ListenRequest
    console.log(`[Listen] Waiting for ${timeout}s...`);
    return '';
  }

  /**
   * 获取用户信息
   */
  async getUserInfo(): Promise<any> {
    // TODO: 调用 gRPC GetUserInfoRequest
    return {
      userId: this.userId,
      nickname: 'User',
      avatar: '',
    };
  }

  /**
   * 获取群组信息
   */
  async getGroupInfo(): Promise<any> {
    if (!this.groupId) {
      return null;
    }

    // TODO: 调用 gRPC GetGroupInfoRequest
    return {
      groupId: this.groupId,
      name: 'Group',
      memberCount: 0,
    };
  }

  /**
   * @某人（QQ/微信支持）
   */
  async atUser(userId: string): Promise<boolean> {
    if (!['qq', 'wechat'].includes(this.platform)) {
      return false;
    }

    // TODO: 调用 gRPC AtUserRequest
    console.log(`[AtUser] @${userId}`);
    return true;
  }

  /**
   * 数据存储
   */
  public storage = {
    get: async (key: string): Promise<string | null> => {
      // TODO: 调用 gRPC StorageGetRequest
      return null;
    },

    set: async (key: string, value: string): Promise<boolean> => {
      // TODO: 调用 gRPC StorageSetRequest
      return true;
    },
  };

  /**
   * HTTP 请求
   */
  public http = {
    get: async (url: string, headers?: Record<string, string>): Promise<any> => {
      // TODO: 调用 gRPC HttpGetRequest
      return { statusCode: 200, body: '', error: '' };
    },

    post: async (url: string, data: string, headers?: Record<string, string>): Promise<any> => {
      // TODO: 调用 gRPC HttpPostRequest
      return { statusCode: 200, body: '', error: '' };
    },
  };
}
