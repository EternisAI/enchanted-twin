/* eslint-disable */
import { TypedDocumentNode as DocumentNode } from '@graphql-typed-document-node/core';
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  DateTime: { input: any; output: any; }
  JSON: { input: any; output: any; }
};

export type AddTrackedFolderInput = {
  name?: InputMaybe<Scalars['String']['input']>;
  path: Scalars['String']['input'];
};

export type AgentTask = {
  __typename?: 'AgentTask';
  completedAt?: Maybe<Scalars['DateTime']['output']>;
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  notify: Scalars['Boolean']['output'];
  output?: Maybe<Scalars['String']['output']>;
  plan?: Maybe<Scalars['String']['output']>;
  previousRuns: Array<Scalars['DateTime']['output']>;
  schedule: Scalars['String']['output'];
  terminatedAt?: Maybe<Scalars['DateTime']['output']>;
  upcomingRuns: Array<Scalars['DateTime']['output']>;
  updatedAt: Scalars['DateTime']['output'];
};

export type AppNotification = {
  __typename?: 'AppNotification';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  image?: Maybe<Scalars['String']['output']>;
  link?: Maybe<Scalars['String']['output']>;
  message: Scalars['String']['output'];
  title: Scalars['String']['output'];
};

export type Author = {
  __typename?: 'Author';
  alias?: Maybe<Scalars['String']['output']>;
  identity: Scalars['String']['output'];
};

export type Chat = {
  __typename?: 'Chat';
  category: ChatCategory;
  createdAt: Scalars['DateTime']['output'];
  holonThreadId?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  initialMessage?: Maybe<Scalars['String']['output']>;
  messages: Array<Message>;
  name: Scalars['String']['output'];
  privacyDictJson?: Maybe<Scalars['JSON']['output']>;
};

export enum ChatCategory {
  Holon = 'HOLON',
  Text = 'TEXT',
  Voice = 'VOICE'
}

export type ChatSuggestionsCategory = {
  __typename?: 'ChatSuggestionsCategory';
  category: Scalars['String']['output'];
  suggestions: Array<Scalars['String']['output']>;
};

export type ConnectMcpServerInput = {
  args?: InputMaybe<Array<Scalars['String']['input']>>;
  command: Scalars['String']['input'];
  envs?: InputMaybe<Array<KeyValueInput>>;
  name: Scalars['String']['input'];
  type: McpServerType;
};

export type DataSource = {
  __typename?: 'DataSource';
  hasError: Scalars['Boolean']['output'];
  id: Scalars['ID']['output'];
  indexProgress: Scalars['Int']['output'];
  isIndexed: Scalars['Boolean']['output'];
  isProcessed: Scalars['Boolean']['output'];
  name: Scalars['String']['output'];
  path: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type DirectoryWatcherStatus = {
  __typename?: 'DirectoryWatcherStatus';
  errorMessage?: Maybe<Scalars['String']['output']>;
  isRunning: Scalars['Boolean']['output'];
  trackedFoldersFromDB: Array<TrackedFolder>;
  watchedDirectories: Array<Scalars['String']['output']>;
};

export enum IndexingState {
  Completed = 'COMPLETED',
  DownloadingModel = 'DOWNLOADING_MODEL',
  Failed = 'FAILED',
  IndexingData = 'INDEXING_DATA',
  NotStarted = 'NOT_STARTED',
  ProcessingData = 'PROCESSING_DATA'
}

export type IndexingStatus = {
  __typename?: 'IndexingStatus';
  dataSources: Array<DataSource>;
  error?: Maybe<Scalars['String']['output']>;
  status: IndexingState;
};

export type KeyValue = {
  __typename?: 'KeyValue';
  key: Scalars['String']['output'];
  value: Scalars['String']['output'];
};

export type KeyValueInput = {
  key: Scalars['String']['input'];
  value: Scalars['String']['input'];
};

export type McpServer = {
  __typename?: 'MCPServer';
  args?: Maybe<Array<Scalars['String']['output']>>;
  command: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  enabled: Scalars['Boolean']['output'];
  envs?: Maybe<Array<KeyValue>>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  type: McpServerType;
};

export type McpServerDefinition = {
  __typename?: 'MCPServerDefinition';
  args?: Maybe<Array<Scalars['String']['output']>>;
  command: Scalars['String']['output'];
  connected: Scalars['Boolean']['output'];
  enabled: Scalars['Boolean']['output'];
  envs?: Maybe<Array<KeyValue>>;
  id: Scalars['String']['output'];
  name: Scalars['String']['output'];
  tools?: Maybe<Array<Tool>>;
  type: McpServerType;
};

export enum McpServerType {
  Enchanted = 'ENCHANTED',
  Freysa = 'FREYSA',
  Google = 'GOOGLE',
  Other = 'OTHER',
  Screenpipe = 'SCREENPIPE',
  Slack = 'SLACK',
  Twitter = 'TWITTER'
}

export type Message = {
  __typename?: 'Message';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  imageUrls: Array<Scalars['String']['output']>;
  role: Role;
  text?: Maybe<Scalars['String']['output']>;
  toolCalls: Array<ToolCall>;
  toolResults: Array<Scalars['String']['output']>;
};

export type MessageInput = {
  role: Role;
  text: Scalars['String']['input'];
};

export type MessageStreamPayload = {
  __typename?: 'MessageStreamPayload';
  accumulatedMessage: Scalars['String']['output'];
  chunk: Scalars['String']['output'];
  createdAt?: Maybe<Scalars['DateTime']['output']>;
  deanonymizedAccumulatedMessage: Scalars['String']['output'];
  imageUrls: Array<Scalars['String']['output']>;
  isComplete: Scalars['Boolean']['output'];
  messageId: Scalars['ID']['output'];
  role: Role;
};

export type Mutation = {
  __typename?: 'Mutation';
  activate: Scalars['Boolean']['output'];
  addDataSource: Scalars['Boolean']['output'];
  addTrackedFolder: TrackedFolder;
  completeOAuthFlow: Scalars['String']['output'];
  connectMCPServer: Scalars['Boolean']['output'];
  createChat: Chat;
  deleteAgentTask: Scalars['Boolean']['output'];
  deleteChat: Chat;
  deleteDataSource: Scalars['Boolean']['output'];
  deleteTrackedFolder: Scalars['Boolean']['output'];
  joinHolon: Scalars['Boolean']['output'];
  refreshExpiredOAuthTokens: Array<OAuthStatus>;
  removeMCPServer: Scalars['Boolean']['output'];
  sendMessage: Message;
  startIndexing: Scalars['Boolean']['output'];
  startOAuthFlow: OAuthFlow;
  startWhatsAppConnection: Scalars['Boolean']['output'];
  storeToken: Scalars['Boolean']['output'];
  updateAgentTask: Scalars['Boolean']['output'];
  updateProfile: Scalars['Boolean']['output'];
  updateTrackedFolder: Scalars['Boolean']['output'];
};


export type MutationActivateArgs = {
  inviteCode: Scalars['String']['input'];
};


export type MutationAddDataSourceArgs = {
  name: Scalars['String']['input'];
  path: Scalars['String']['input'];
};


export type MutationAddTrackedFolderArgs = {
  input: AddTrackedFolderInput;
};


export type MutationCompleteOAuthFlowArgs = {
  authCode: Scalars['String']['input'];
  state: Scalars['String']['input'];
};


export type MutationConnectMcpServerArgs = {
  input: ConnectMcpServerInput;
};


export type MutationCreateChatArgs = {
  category?: ChatCategory;
  holonThreadId?: InputMaybe<Scalars['String']['input']>;
  initialMessage?: InputMaybe<Scalars['String']['input']>;
  isReasoning?: Scalars['Boolean']['input'];
  name: Scalars['String']['input'];
};


export type MutationDeleteAgentTaskArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteChatArgs = {
  chatId: Scalars['ID']['input'];
};


export type MutationDeleteDataSourceArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteTrackedFolderArgs = {
  id: Scalars['ID']['input'];
};


export type MutationJoinHolonArgs = {
  network?: InputMaybe<Scalars['String']['input']>;
  userId: Scalars['String']['input'];
};


export type MutationRemoveMcpServerArgs = {
  id: Scalars['String']['input'];
};


export type MutationSendMessageArgs = {
  chatId: Scalars['ID']['input'];
  reasoning: Scalars['Boolean']['input'];
  text: Scalars['String']['input'];
  voice: Scalars['Boolean']['input'];
};


export type MutationStartOAuthFlowArgs = {
  provider: Scalars['String']['input'];
  scope: Scalars['String']['input'];
};


export type MutationStoreTokenArgs = {
  input: StoreTokenInput;
};


export type MutationUpdateAgentTaskArgs = {
  id: Scalars['ID']['input'];
  notify: Scalars['Boolean']['input'];
};


export type MutationUpdateProfileArgs = {
  input: UpdateProfileInput;
};


export type MutationUpdateTrackedFolderArgs = {
  id: Scalars['ID']['input'];
  input: UpdateTrackedFolderInput;
};

export type OAuthAccount = {
  __typename?: 'OAuthAccount';
  expiresAt: Scalars['DateTime']['output'];
  isActive: Scalars['Boolean']['output'];
  provider: Scalars['String']['output'];
  username: Scalars['String']['output'];
};

export type OAuthFlow = {
  __typename?: 'OAuthFlow';
  authURL: Scalars['String']['output'];
  redirectURI: Scalars['String']['output'];
};

export type OAuthStatus = {
  __typename?: 'OAuthStatus';
  error: Scalars['Boolean']['output'];
  expiresAt: Scalars['DateTime']['output'];
  provider: Scalars['String']['output'];
  scope: Array<Scalars['String']['output']>;
  username: Scalars['String']['output'];
};

export type PrivacyDictUpdate = {
  __typename?: 'PrivacyDictUpdate';
  chatId: Scalars['ID']['output'];
  privacyDictJson: Scalars['JSON']['output'];
};

export type Query = {
  __typename?: 'Query';
  getAgentTasks: Array<AgentTask>;
  getChat: Chat;
  getChatSuggestions: Array<ChatSuggestionsCategory>;
  getChats: Array<Chat>;
  getConnectedAccounts: Array<OAuthAccount>;
  getDataSources: Array<DataSource>;
  getDirectoryWatcherStatus: DirectoryWatcherStatus;
  getHolons?: Maybe<Array<Scalars['String']['output']>>;
  getMCPServers: Array<McpServerDefinition>;
  getOAuthStatus: Array<OAuthStatus>;
  getSetupProgress: Array<SetupProgress>;
  getThread?: Maybe<Thread>;
  getThreads: Array<Thread>;
  getTools: Array<Tool>;
  getTrackedFolders: Array<TrackedFolder>;
  getWhatsAppStatus: WhatsAppStatus;
  profile: UserProfile;
  whitelistStatus: Scalars['Boolean']['output'];
};


export type QueryGetChatArgs = {
  id: Scalars['ID']['input'];
};


export type QueryGetChatSuggestionsArgs = {
  chatId: Scalars['ID']['input'];
};


export type QueryGetChatsArgs = {
  first?: Scalars['Int']['input'];
  offset?: Scalars['Int']['input'];
};


export type QueryGetHolonsArgs = {
  userId: Scalars['ID']['input'];
};


export type QueryGetThreadArgs = {
  id: Scalars['ID']['input'];
  network?: InputMaybe<Scalars['String']['input']>;
};


export type QueryGetThreadsArgs = {
  first?: Scalars['Int']['input'];
  network?: InputMaybe<Scalars['String']['input']>;
  offset?: Scalars['Int']['input'];
};

export enum Role {
  Assistant = 'ASSISTANT',
  User = 'USER'
}

export type SetupProgress = {
  __typename?: 'SetupProgress';
  name: Scalars['String']['output'];
  progress: Scalars['Float']['output'];
  required: Scalars['Boolean']['output'];
  status: Scalars['String']['output'];
};

export type StoreTokenInput = {
  refreshToken: Scalars['String']['input'];
  token: Scalars['String']['input'];
};

export type Subscription = {
  __typename?: 'Subscription';
  indexingStatus: IndexingStatus;
  messageAdded: Message;
  messageStream: MessageStreamPayload;
  notificationAdded: AppNotification;
  privacyDictUpdated: PrivacyDictUpdate;
  processMessageHistoryStream: MessageStreamPayload;
  toolCallUpdated: ToolCall;
  whatsAppSyncStatus: WhatsAppSyncStatus;
};


export type SubscriptionMessageAddedArgs = {
  chatId: Scalars['ID']['input'];
};


export type SubscriptionMessageStreamArgs = {
  chatId: Scalars['ID']['input'];
};


export type SubscriptionPrivacyDictUpdatedArgs = {
  chatId: Scalars['ID']['input'];
};


export type SubscriptionProcessMessageHistoryStreamArgs = {
  chatId: Scalars['ID']['input'];
  isOnboarding: Scalars['Boolean']['input'];
  messages: Array<MessageInput>;
};


export type SubscriptionToolCallUpdatedArgs = {
  chatId: Scalars['ID']['input'];
};

export type Thread = {
  __typename?: 'Thread';
  actions?: Maybe<Array<Scalars['String']['output']>>;
  author: Author;
  content: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  expiresAt?: Maybe<Scalars['DateTime']['output']>;
  id: Scalars['ID']['output'];
  imageURLs: Array<Scalars['String']['output']>;
  messages: Array<ThreadMessage>;
  remoteThreadId?: Maybe<Scalars['Int']['output']>;
  title: Scalars['String']['output'];
  views: Scalars['Int']['output'];
};

export type ThreadMessage = {
  __typename?: 'ThreadMessage';
  actions?: Maybe<Array<Scalars['String']['output']>>;
  author: Author;
  content: Scalars['String']['output'];
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  isDelivered?: Maybe<Scalars['Boolean']['output']>;
  state: Scalars['String']['output'];
};

export type Tool = {
  __typename?: 'Tool';
  description: Scalars['String']['output'];
  name: Scalars['String']['output'];
};

export type ToolCall = {
  __typename?: 'ToolCall';
  error?: Maybe<Scalars['String']['output']>;
  id: Scalars['String']['output'];
  isCompleted: Scalars['Boolean']['output'];
  messageId: Scalars['String']['output'];
  name: Scalars['String']['output'];
  result?: Maybe<ToolCallResult>;
};

export type ToolCallResult = {
  __typename?: 'ToolCallResult';
  content?: Maybe<Scalars['String']['output']>;
  imageUrls: Array<Scalars['String']['output']>;
};

export type TrackedFolder = {
  __typename?: 'TrackedFolder';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  isEnabled: Scalars['Boolean']['output'];
  name?: Maybe<Scalars['String']['output']>;
  path: Scalars['String']['output'];
  updatedAt: Scalars['DateTime']['output'];
};

export type UpdateProfileInput = {
  bio?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateTrackedFolderInput = {
  isEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
};

export type UserProfile = {
  __typename?: 'UserProfile';
  bio?: Maybe<Scalars['String']['output']>;
  connectedDataSources: Array<DataSource>;
  indexingStatus?: Maybe<IndexingStatus>;
  name?: Maybe<Scalars['String']['output']>;
  username?: Maybe<Scalars['String']['output']>;
};

export type WhatsAppQrCodeUpdate = {
  __typename?: 'WhatsAppQRCodeUpdate';
  event: Scalars['String']['output'];
  isConnected: Scalars['Boolean']['output'];
  qrCodeData?: Maybe<Scalars['String']['output']>;
  timestamp: Scalars['DateTime']['output'];
};

export type WhatsAppStatus = {
  __typename?: 'WhatsAppStatus';
  isConnected: Scalars['Boolean']['output'];
  qrCodeData?: Maybe<Scalars['String']['output']>;
  statusMessage: Scalars['String']['output'];
};

export type WhatsAppSyncStatus = {
  __typename?: 'WhatsAppSyncStatus';
  error?: Maybe<Scalars['String']['output']>;
  isCompleted: Scalars['Boolean']['output'];
  isSyncing: Scalars['Boolean']['output'];
  statusMessage?: Maybe<Scalars['String']['output']>;
};

export type GetProfileQueryVariables = Exact<{ [key: string]: never; }>;


export type GetProfileQuery = { __typename?: 'Query', profile: { __typename?: 'UserProfile', name?: string | null } };

export type GetChatsQueryVariables = Exact<{
  first: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
}>;


export type GetChatsQuery = { __typename?: 'Query', getChats: Array<{ __typename?: 'Chat', id: string, name: string, createdAt: any, category: ChatCategory, holonThreadId?: string | null, privacyDictJson?: any | null, messages: Array<{ __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string, error?: string | null, result?: { __typename?: 'ToolCallResult', content?: string | null, imageUrls: Array<string> } | null }> }> }> };

export type GetChatQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type GetChatQuery = { __typename?: 'Query', getChat: { __typename?: 'Chat', id: string, name: string, createdAt: any, category: ChatCategory, holonThreadId?: string | null, privacyDictJson?: any | null, messages: Array<{ __typename?: 'Message', id: string, text?: string | null, imageUrls: Array<string>, role: Role, createdAt: any, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string, error?: string | null, result?: { __typename?: 'ToolCallResult', content?: string | null, imageUrls: Array<string> } | null }> }> } };

export type GetDataSourcesQueryVariables = Exact<{ [key: string]: never; }>;


export type GetDataSourcesQuery = { __typename?: 'Query', getDataSources: Array<{ __typename?: 'DataSource', id: string, name: string, path: string, updatedAt: any, isProcessed: boolean, isIndexed: boolean, indexProgress: number, hasError: boolean }> };

export type GetOAuthStatusQueryVariables = Exact<{ [key: string]: never; }>;


export type GetOAuthStatusQuery = { __typename?: 'Query', getOAuthStatus: Array<{ __typename?: 'OAuthStatus', provider: string, expiresAt: any, scope: Array<string> }> };

export type GetChatSuggestionsQueryVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type GetChatSuggestionsQuery = { __typename?: 'Query', getChatSuggestions: Array<{ __typename?: 'ChatSuggestionsCategory', category: string, suggestions: Array<string> }> };

export type GetMcpServersQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMcpServersQuery = { __typename?: 'Query', getMCPServers: Array<{ __typename?: 'MCPServerDefinition', id: string, name: string, command: string, args?: Array<string> | null, type: McpServerType, enabled: boolean, connected: boolean, envs?: Array<{ __typename?: 'KeyValue', key: string, value: string }> | null }> };

export type GetAgentTasksQueryVariables = Exact<{ [key: string]: never; }>;


export type GetAgentTasksQuery = { __typename?: 'Query', getAgentTasks: Array<{ __typename?: 'AgentTask', id: string, name: string, schedule: string, plan?: string | null, createdAt: any, updatedAt: any, completedAt?: any | null, terminatedAt?: any | null, output?: string | null, notify: boolean }> };

export type GetToolsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetToolsQuery = { __typename?: 'Query', getTools: Array<{ __typename?: 'Tool', name: string, description: string }> };

export type GetSetupProgressQueryVariables = Exact<{ [key: string]: never; }>;


export type GetSetupProgressQuery = { __typename?: 'Query', getSetupProgress: Array<{ __typename?: 'SetupProgress', name: string, status: string, progress: number, required: boolean }> };

export type GetWhitelistStatusQueryVariables = Exact<{ [key: string]: never; }>;


export type GetWhitelistStatusQuery = { __typename?: 'Query', whitelistStatus: boolean };

export type GetTrackedFoldersQueryVariables = Exact<{ [key: string]: never; }>;


export type GetTrackedFoldersQuery = { __typename?: 'Query', getTrackedFolders: Array<{ __typename?: 'TrackedFolder', id: string, name?: string | null, path: string, isEnabled: boolean, createdAt: any, updatedAt: any }> };

export type GetDirectoryWatcherStatusQueryVariables = Exact<{ [key: string]: never; }>;


export type GetDirectoryWatcherStatusQuery = { __typename?: 'Query', getDirectoryWatcherStatus: { __typename?: 'DirectoryWatcherStatus', isRunning: boolean, watchedDirectories: Array<string>, errorMessage?: string | null, trackedFoldersFromDB: Array<{ __typename?: 'TrackedFolder', id: string, name?: string | null, path: string, isEnabled: boolean, createdAt: any, updatedAt: any }> } };

export type CreateChatMutationVariables = Exact<{
  name: Scalars['String']['input'];
  category?: ChatCategory;
  holonThreadId?: InputMaybe<Scalars['String']['input']>;
  initialMessage?: InputMaybe<Scalars['String']['input']>;
  isReasoning?: Scalars['Boolean']['input'];
}>;


export type CreateChatMutation = { __typename?: 'Mutation', createChat: { __typename?: 'Chat', id: string, name: string, createdAt: any, category: ChatCategory, holonThreadId?: string | null, privacyDictJson?: any | null } };

export type SendMessageMutationVariables = Exact<{
  chatId: Scalars['ID']['input'];
  text: Scalars['String']['input'];
  reasoning: Scalars['Boolean']['input'];
  voice: Scalars['Boolean']['input'];
}>;


export type SendMessageMutation = { __typename?: 'Mutation', sendMessage: { __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, error?: string | null, result?: { __typename?: 'ToolCallResult', content?: string | null, imageUrls: Array<string> } | null }> } };

export type DeleteChatMutationVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type DeleteChatMutation = { __typename?: 'Mutation', deleteChat: { __typename?: 'Chat', id: string, name: string } };

export type UpdateProfileMutationVariables = Exact<{
  input: UpdateProfileInput;
}>;


export type UpdateProfileMutation = { __typename?: 'Mutation', updateProfile: boolean };

export type ActivateInviteCodeMutationVariables = Exact<{
  inviteCode: Scalars['String']['input'];
}>;


export type ActivateInviteCodeMutation = { __typename?: 'Mutation', activate: boolean };

export type MessageAddedSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type MessageAddedSubscription = { __typename?: 'Subscription', messageAdded: { __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string, error?: string | null, result?: { __typename?: 'ToolCallResult', content?: string | null, imageUrls: Array<string> } | null }> } };

export type ToolCallUpdatedSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type ToolCallUpdatedSubscription = { __typename?: 'Subscription', toolCallUpdated: { __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string, error?: string | null, result?: { __typename?: 'ToolCallResult', content?: string | null, imageUrls: Array<string> } | null } };

export type IndexingStatusSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type IndexingStatusSubscription = { __typename?: 'Subscription', indexingStatus: { __typename?: 'IndexingStatus', status: IndexingState, error?: string | null, dataSources: Array<{ __typename?: 'DataSource', id: string, name: string, isProcessed: boolean, isIndexed: boolean, indexProgress: number, hasError: boolean }> } };

export type NotificationAddedSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type NotificationAddedSubscription = { __typename?: 'Subscription', notificationAdded: { __typename?: 'AppNotification', id: string, title: string, message: string, image?: string | null, link?: string | null, createdAt: any } };

export type MessageStreamSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type MessageStreamSubscription = { __typename?: 'Subscription', messageStream: { __typename?: 'MessageStreamPayload', messageId: string, chunk: string, role: Role, isComplete: boolean, createdAt?: any | null, imageUrls: Array<string>, accumulatedMessage: string, deanonymizedAccumulatedMessage: string } };

export type WhatsAppSyncStatusSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type WhatsAppSyncStatusSubscription = { __typename?: 'Subscription', whatsAppSyncStatus: { __typename?: 'WhatsAppSyncStatus', isSyncing: boolean, isCompleted: boolean, error?: string | null, statusMessage?: string | null } };

export type PrivacyDictUpdatedSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type PrivacyDictUpdatedSubscription = { __typename?: 'Subscription', privacyDictUpdated: { __typename?: 'PrivacyDictUpdate', privacyDictJson: any } };

export type ProcessMessageHistoryStreamSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
  messages: Array<MessageInput> | MessageInput;
  isOnboarding: Scalars['Boolean']['input'];
}>;


export type ProcessMessageHistoryStreamSubscription = { __typename?: 'Subscription', processMessageHistoryStream: { __typename?: 'MessageStreamPayload', messageId: string, chunk: string, role: Role, isComplete: boolean, createdAt?: any | null, imageUrls: Array<string> } };

export type AddDataSourceMutationVariables = Exact<{
  name: Scalars['String']['input'];
  path: Scalars['String']['input'];
}>;


export type AddDataSourceMutation = { __typename?: 'Mutation', addDataSource: boolean };

export type DeleteDataSourceMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteDataSourceMutation = { __typename?: 'Mutation', deleteDataSource: boolean };

export type StartOAuthFlowMutationVariables = Exact<{
  provider: Scalars['String']['input'];
  scope: Scalars['String']['input'];
}>;


export type StartOAuthFlowMutation = { __typename?: 'Mutation', startOAuthFlow: { __typename?: 'OAuthFlow', authURL: string, redirectURI: string } };

export type CompleteOAuthFlowMutationVariables = Exact<{
  state: Scalars['String']['input'];
  authCode: Scalars['String']['input'];
}>;


export type CompleteOAuthFlowMutation = { __typename?: 'Mutation', completeOAuthFlow: string };

export type DeleteAgentTaskMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteAgentTaskMutation = { __typename?: 'Mutation', deleteAgentTask: boolean };

export type UpdateAgentTaskMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  notify: Scalars['Boolean']['input'];
}>;


export type UpdateAgentTaskMutation = { __typename?: 'Mutation', updateAgentTask: boolean };

export type ConnectMcpServerMutationVariables = Exact<{
  input: ConnectMcpServerInput;
}>;


export type ConnectMcpServerMutation = { __typename?: 'Mutation', connectMCPServer: boolean };

export type RemoveMcpServerMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type RemoveMcpServerMutation = { __typename?: 'Mutation', removeMCPServer: boolean };

export type GetWhatsAppStatusQueryVariables = Exact<{ [key: string]: never; }>;


export type GetWhatsAppStatusQuery = { __typename?: 'Query', getWhatsAppStatus: { __typename?: 'WhatsAppStatus', isConnected: boolean, qrCodeData?: string | null, statusMessage: string } };

export type GetThreadsQueryVariables = Exact<{
  network?: InputMaybe<Scalars['String']['input']>;
  first?: Scalars['Int']['input'];
  offset?: Scalars['Int']['input'];
}>;


export type GetThreadsQuery = { __typename?: 'Query', getThreads: Array<{ __typename?: 'Thread', id: string, title: string, content: string, imageURLs: Array<string>, createdAt: any, expiresAt?: any | null, actions?: Array<string> | null, views: number, author: { __typename?: 'Author', alias?: string | null, identity: string }, messages: Array<{ __typename?: 'ThreadMessage', id: string, content: string, createdAt: any, isDelivered?: boolean | null, actions?: Array<string> | null, state: string, author: { __typename?: 'Author', alias?: string | null, identity: string } }> }> };

export type GetThreadQueryVariables = Exact<{
  network?: InputMaybe<Scalars['String']['input']>;
  id: Scalars['ID']['input'];
}>;


export type GetThreadQuery = { __typename?: 'Query', getThread?: { __typename?: 'Thread', id: string, title: string, content: string, imageURLs: Array<string>, createdAt: any, expiresAt?: any | null, actions?: Array<string> | null, views: number, author: { __typename?: 'Author', alias?: string | null, identity: string }, messages: Array<{ __typename?: 'ThreadMessage', id: string, content: string, createdAt: any, isDelivered?: boolean | null, actions?: Array<string> | null, state: string, author: { __typename?: 'Author', alias?: string | null, identity: string } }> } | null };

export type GetHolonsQueryVariables = Exact<{
  userId: Scalars['ID']['input'];
}>;


export type GetHolonsQuery = { __typename?: 'Query', getHolons?: Array<string> | null };

export type JoinHolonMutationVariables = Exact<{
  userId: Scalars['String']['input'];
  network?: InputMaybe<Scalars['String']['input']>;
}>;


export type JoinHolonMutation = { __typename?: 'Mutation', joinHolon: boolean };

export type StartWhatsAppConnectionMutationVariables = Exact<{ [key: string]: never; }>;


export type StartWhatsAppConnectionMutation = { __typename?: 'Mutation', startWhatsAppConnection: boolean };

export type StoreTokenMutationVariables = Exact<{
  input: StoreTokenInput;
}>;


export type StoreTokenMutation = { __typename?: 'Mutation', storeToken: boolean };

export type AddTrackedFolderMutationVariables = Exact<{
  input: AddTrackedFolderInput;
}>;


export type AddTrackedFolderMutation = { __typename?: 'Mutation', addTrackedFolder: { __typename?: 'TrackedFolder', id: string, name?: string | null, path: string, isEnabled: boolean, createdAt: any, updatedAt: any } };

export type DeleteTrackedFolderMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteTrackedFolderMutation = { __typename?: 'Mutation', deleteTrackedFolder: boolean };

export type UpdateTrackedFolderMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateTrackedFolderInput;
}>;


export type UpdateTrackedFolderMutation = { __typename?: 'Mutation', updateTrackedFolder: boolean };


export const GetProfileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetProfile"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"profile"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<GetProfileQuery, GetProfileQueryVariables>;
export const GetChatsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChats"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"first"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"offset"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChats"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"first"},"value":{"kind":"Variable","name":{"kind":"Name","value":"first"}}},{"kind":"Argument","name":{"kind":"Name","value":"offset"},"value":{"kind":"Variable","name":{"kind":"Name","value":"offset"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"category"}},{"kind":"Field","name":{"kind":"Name","value":"holonThreadId"}},{"kind":"Field","name":{"kind":"Name","value":"privacyDictJson"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"result"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}}]}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}}]}}]}}]}}]} as unknown as DocumentNode<GetChatsQuery, GetChatsQueryVariables>;
export const GetChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"category"}},{"kind":"Field","name":{"kind":"Name","value":"holonThreadId"}},{"kind":"Field","name":{"kind":"Name","value":"privacyDictJson"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"result"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}}]}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}}]}}]}}]}}]} as unknown as DocumentNode<GetChatQuery, GetChatQueryVariables>;
export const GetDataSourcesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetDataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getDataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"isProcessed"}},{"kind":"Field","name":{"kind":"Name","value":"isIndexed"}},{"kind":"Field","name":{"kind":"Name","value":"indexProgress"}},{"kind":"Field","name":{"kind":"Name","value":"hasError"}}]}}]}}]} as unknown as DocumentNode<GetDataSourcesQuery, GetDataSourcesQueryVariables>;
export const GetOAuthStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetOAuthStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getOAuthStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"provider"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"scope"}}]}}]}}]} as unknown as DocumentNode<GetOAuthStatusQuery, GetOAuthStatusQueryVariables>;
export const GetChatSuggestionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChatSuggestions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChatSuggestions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"category"}},{"kind":"Field","name":{"kind":"Name","value":"suggestions"}}]}}]}}]} as unknown as DocumentNode<GetChatSuggestionsQuery, GetChatSuggestionsQueryVariables>;
export const GetMcpServersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetMCPServers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getMCPServers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"command"}},{"kind":"Field","name":{"kind":"Name","value":"args"}},{"kind":"Field","name":{"kind":"Name","value":"envs"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"connected"}}]}}]}}]} as unknown as DocumentNode<GetMcpServersQuery, GetMcpServersQueryVariables>;
export const GetAgentTasksDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetAgentTasks"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getAgentTasks"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"schedule"}},{"kind":"Field","name":{"kind":"Name","value":"plan"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"completedAt"}},{"kind":"Field","name":{"kind":"Name","value":"terminatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"output"}},{"kind":"Field","name":{"kind":"Name","value":"notify"}}]}}]}}]} as unknown as DocumentNode<GetAgentTasksQuery, GetAgentTasksQueryVariables>;
export const GetToolsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetTools"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getTools"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]} as unknown as DocumentNode<GetToolsQuery, GetToolsQueryVariables>;
export const GetSetupProgressDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetSetupProgress"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getSetupProgress"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"progress"}},{"kind":"Field","name":{"kind":"Name","value":"required"}}]}}]}}]} as unknown as DocumentNode<GetSetupProgressQuery, GetSetupProgressQueryVariables>;
export const GetWhitelistStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetWhitelistStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"whitelistStatus"}}]}}]} as unknown as DocumentNode<GetWhitelistStatusQuery, GetWhitelistStatusQueryVariables>;
export const GetTrackedFoldersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetTrackedFolders"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getTrackedFolders"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"isEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}}]}}]}}]} as unknown as DocumentNode<GetTrackedFoldersQuery, GetTrackedFoldersQueryVariables>;
export const GetDirectoryWatcherStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetDirectoryWatcherStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getDirectoryWatcherStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"isRunning"}},{"kind":"Field","name":{"kind":"Name","value":"watchedDirectories"}},{"kind":"Field","name":{"kind":"Name","value":"trackedFoldersFromDB"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"isEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}}]}},{"kind":"Field","name":{"kind":"Name","value":"errorMessage"}}]}}]}}]} as unknown as DocumentNode<GetDirectoryWatcherStatusQuery, GetDirectoryWatcherStatusQueryVariables>;
export const CreateChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"category"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ChatCategory"}}},"defaultValue":{"kind":"EnumValue","value":"TEXT"}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"holonThreadId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"initialMessage"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"isReasoning"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}},"defaultValue":{"kind":"BooleanValue","value":false}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"category"},"value":{"kind":"Variable","name":{"kind":"Name","value":"category"}}},{"kind":"Argument","name":{"kind":"Name","value":"holonThreadId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"holonThreadId"}}},{"kind":"Argument","name":{"kind":"Name","value":"initialMessage"},"value":{"kind":"Variable","name":{"kind":"Name","value":"initialMessage"}}},{"kind":"Argument","name":{"kind":"Name","value":"isReasoning"},"value":{"kind":"Variable","name":{"kind":"Name","value":"isReasoning"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"category"}},{"kind":"Field","name":{"kind":"Name","value":"holonThreadId"}},{"kind":"Field","name":{"kind":"Name","value":"privacyDictJson"}}]}}]}}]} as unknown as DocumentNode<CreateChatMutation, CreateChatMutationVariables>;
export const SendMessageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SendMessage"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"text"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"reasoning"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"voice"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"sendMessage"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}},{"kind":"Argument","name":{"kind":"Name","value":"text"},"value":{"kind":"Variable","name":{"kind":"Name","value":"text"}}},{"kind":"Argument","name":{"kind":"Name","value":"reasoning"},"value":{"kind":"Variable","name":{"kind":"Name","value":"reasoning"}}},{"kind":"Argument","name":{"kind":"Name","value":"voice"},"value":{"kind":"Variable","name":{"kind":"Name","value":"voice"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"result"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}}]}}]}}]}}]} as unknown as DocumentNode<SendMessageMutation, SendMessageMutationVariables>;
export const DeleteChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<DeleteChatMutation, DeleteChatMutationVariables>;
export const UpdateProfileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateProfile"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"UpdateProfileInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateProfile"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}]}]}}]} as unknown as DocumentNode<UpdateProfileMutation, UpdateProfileMutationVariables>;
export const ActivateInviteCodeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ActivateInviteCode"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"inviteCode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"activate"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"inviteCode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"inviteCode"}}}]}]}}]} as unknown as DocumentNode<ActivateInviteCodeMutation, ActivateInviteCodeMutationVariables>;
export const MessageAddedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"MessageAdded"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"messageAdded"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"result"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}}]}}]}}]}}]} as unknown as DocumentNode<MessageAddedSubscription, MessageAddedSubscriptionVariables>;
export const ToolCallUpdatedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"ToolCallUpdated"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"toolCallUpdated"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"result"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}}]}}]}}]} as unknown as DocumentNode<ToolCallUpdatedSubscription, ToolCallUpdatedSubscriptionVariables>;
export const IndexingStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"IndexingStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"indexingStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"error"}},{"kind":"Field","name":{"kind":"Name","value":"dataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isProcessed"}},{"kind":"Field","name":{"kind":"Name","value":"isIndexed"}},{"kind":"Field","name":{"kind":"Name","value":"indexProgress"}},{"kind":"Field","name":{"kind":"Name","value":"hasError"}}]}}]}}]}}]} as unknown as DocumentNode<IndexingStatusSubscription, IndexingStatusSubscriptionVariables>;
export const NotificationAddedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"NotificationAdded"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"notificationAdded"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"message"}},{"kind":"Field","name":{"kind":"Name","value":"image"}},{"kind":"Field","name":{"kind":"Name","value":"link"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<NotificationAddedSubscription, NotificationAddedSubscriptionVariables>;
export const MessageStreamDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"MessageStream"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"messageStream"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"chunk"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"isComplete"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"accumulatedMessage"}},{"kind":"Field","name":{"kind":"Name","value":"deanonymizedAccumulatedMessage"}}]}}]}}]} as unknown as DocumentNode<MessageStreamSubscription, MessageStreamSubscriptionVariables>;
export const WhatsAppSyncStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"WhatsAppSyncStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"whatsAppSyncStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"isSyncing"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"error"}},{"kind":"Field","name":{"kind":"Name","value":"statusMessage"}}]}}]}}]} as unknown as DocumentNode<WhatsAppSyncStatusSubscription, WhatsAppSyncStatusSubscriptionVariables>;
export const PrivacyDictUpdatedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"PrivacyDictUpdated"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"privacyDictUpdated"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"privacyDictJson"}}]}}]}}]} as unknown as DocumentNode<PrivacyDictUpdatedSubscription, PrivacyDictUpdatedSubscriptionVariables>;
export const ProcessMessageHistoryStreamDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"ProcessMessageHistoryStream"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"messages"}},"type":{"kind":"NonNullType","type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"MessageInput"}}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"isOnboarding"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"processMessageHistoryStream"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}},{"kind":"Argument","name":{"kind":"Name","value":"messages"},"value":{"kind":"Variable","name":{"kind":"Name","value":"messages"}}},{"kind":"Argument","name":{"kind":"Name","value":"isOnboarding"},"value":{"kind":"Variable","name":{"kind":"Name","value":"isOnboarding"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"messageId"}},{"kind":"Field","name":{"kind":"Name","value":"chunk"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"isComplete"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}}]}}]}}]} as unknown as DocumentNode<ProcessMessageHistoryStreamSubscription, ProcessMessageHistoryStreamSubscriptionVariables>;
export const AddDataSourceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddDataSource"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addDataSource"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}]}]}}]} as unknown as DocumentNode<AddDataSourceMutation, AddDataSourceMutationVariables>;
export const DeleteDataSourceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDataSource"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDataSource"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteDataSourceMutation, DeleteDataSourceMutationVariables>;
export const StartOAuthFlowDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StartOAuthFlow"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"provider"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"scope"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"startOAuthFlow"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"provider"},"value":{"kind":"Variable","name":{"kind":"Name","value":"provider"}}},{"kind":"Argument","name":{"kind":"Name","value":"scope"},"value":{"kind":"Variable","name":{"kind":"Name","value":"scope"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"authURL"}},{"kind":"Field","name":{"kind":"Name","value":"redirectURI"}}]}}]}}]} as unknown as DocumentNode<StartOAuthFlowMutation, StartOAuthFlowMutationVariables>;
export const CompleteOAuthFlowDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CompleteOAuthFlow"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"state"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"authCode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"completeOAuthFlow"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"state"},"value":{"kind":"Variable","name":{"kind":"Name","value":"state"}}},{"kind":"Argument","name":{"kind":"Name","value":"authCode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"authCode"}}}]}]}}]} as unknown as DocumentNode<CompleteOAuthFlowMutation, CompleteOAuthFlowMutationVariables>;
export const DeleteAgentTaskDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteAgentTask"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteAgentTask"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteAgentTaskMutation, DeleteAgentTaskMutationVariables>;
export const UpdateAgentTaskDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateAgentTask"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"notify"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateAgentTask"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"notify"},"value":{"kind":"Variable","name":{"kind":"Name","value":"notify"}}}]}]}}]} as unknown as DocumentNode<UpdateAgentTaskMutation, UpdateAgentTaskMutationVariables>;
export const ConnectMcpServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ConnectMCPServer"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ConnectMCPServerInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"connectMCPServer"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}]}]}}]} as unknown as DocumentNode<ConnectMcpServerMutation, ConnectMcpServerMutationVariables>;
export const RemoveMcpServerDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RemoveMCPServer"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"removeMCPServer"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<RemoveMcpServerMutation, RemoveMcpServerMutationVariables>;
export const GetWhatsAppStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetWhatsAppStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getWhatsAppStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"isConnected"}},{"kind":"Field","name":{"kind":"Name","value":"qrCodeData"}},{"kind":"Field","name":{"kind":"Name","value":"statusMessage"}}]}}]}}]} as unknown as DocumentNode<GetWhatsAppStatusQuery, GetWhatsAppStatusQueryVariables>;
export const GetThreadsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetThreads"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"network"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"first"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},"defaultValue":{"kind":"IntValue","value":"10"}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"offset"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}},"defaultValue":{"kind":"IntValue","value":"0"}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getThreads"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"network"},"value":{"kind":"Variable","name":{"kind":"Name","value":"network"}}},{"kind":"Argument","name":{"kind":"Name","value":"first"},"value":{"kind":"Variable","name":{"kind":"Name","value":"first"}}},{"kind":"Argument","name":{"kind":"Name","value":"offset"},"value":{"kind":"Variable","name":{"kind":"Name","value":"offset"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageURLs"}},{"kind":"Field","name":{"kind":"Name","value":"author"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"alias"}},{"kind":"Field","name":{"kind":"Name","value":"identity"}}]}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"author"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"alias"}},{"kind":"Field","name":{"kind":"Name","value":"identity"}}]}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"isDelivered"}},{"kind":"Field","name":{"kind":"Name","value":"actions"}},{"kind":"Field","name":{"kind":"Name","value":"state"}}]}},{"kind":"Field","name":{"kind":"Name","value":"actions"}},{"kind":"Field","name":{"kind":"Name","value":"views"}}]}}]}}]} as unknown as DocumentNode<GetThreadsQuery, GetThreadsQueryVariables>;
export const GetThreadDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetThread"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"network"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getThread"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"network"},"value":{"kind":"Variable","name":{"kind":"Name","value":"network"}}},{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"imageURLs"}},{"kind":"Field","name":{"kind":"Name","value":"author"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"alias"}},{"kind":"Field","name":{"kind":"Name","value":"identity"}}]}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"author"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"alias"}},{"kind":"Field","name":{"kind":"Name","value":"identity"}}]}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"isDelivered"}},{"kind":"Field","name":{"kind":"Name","value":"actions"}},{"kind":"Field","name":{"kind":"Name","value":"state"}}]}},{"kind":"Field","name":{"kind":"Name","value":"actions"}},{"kind":"Field","name":{"kind":"Name","value":"views"}}]}}]}}]} as unknown as DocumentNode<GetThreadQuery, GetThreadQueryVariables>;
export const GetHolonsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetHolons"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"userId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getHolons"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"userId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"userId"}}}]}]}}]} as unknown as DocumentNode<GetHolonsQuery, GetHolonsQueryVariables>;
export const JoinHolonDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"JoinHolon"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"userId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"network"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"joinHolon"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"userId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"userId"}}},{"kind":"Argument","name":{"kind":"Name","value":"network"},"value":{"kind":"Variable","name":{"kind":"Name","value":"network"}}}]}]}}]} as unknown as DocumentNode<JoinHolonMutation, JoinHolonMutationVariables>;
export const StartWhatsAppConnectionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StartWhatsAppConnection"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"startWhatsAppConnection"}}]}}]} as unknown as DocumentNode<StartWhatsAppConnectionMutation, StartWhatsAppConnectionMutationVariables>;
export const StoreTokenDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StoreToken"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"StoreTokenInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"storeToken"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}]}]}}]} as unknown as DocumentNode<StoreTokenMutation, StoreTokenMutationVariables>;
export const AddTrackedFolderDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddTrackedFolder"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"AddTrackedFolderInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addTrackedFolder"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"isEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}}]}}]}}]} as unknown as DocumentNode<AddTrackedFolderMutation, AddTrackedFolderMutationVariables>;
export const DeleteTrackedFolderDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteTrackedFolder"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteTrackedFolder"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteTrackedFolderMutation, DeleteTrackedFolderMutationVariables>;
export const UpdateTrackedFolderDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateTrackedFolder"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"UpdateTrackedFolderInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateTrackedFolder"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}]}]}}]} as unknown as DocumentNode<UpdateTrackedFolderMutation, UpdateTrackedFolderMutationVariables>;