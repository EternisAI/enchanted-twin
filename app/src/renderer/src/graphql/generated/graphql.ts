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

export type Chat = {
  __typename?: 'Chat';
  createdAt: Scalars['DateTime']['output'];
  id: Scalars['ID']['output'];
  messages: Array<Message>;
  name: Scalars['String']['output'];
};

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
  type: McpServerType;
};

export enum McpServerType {
  Google = 'GOOGLE',
  Other = 'OTHER',
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

export type Mutation = {
  __typename?: 'Mutation';
  addDataSource: Scalars['Boolean']['output'];
  completeOAuthFlow: Scalars['String']['output'];
  connectMCPServer: Scalars['Boolean']['output'];
  createChat: Chat;
  deleteChat: Chat;
  deleteDataSource: Scalars['Boolean']['output'];
  refreshExpiredOAuthTokens: Array<OAuthStatus>;
  sendMessage: Message;
  sendTelegramMessage: Scalars['Boolean']['output'];
  startIndexing: Scalars['Boolean']['output'];
  startOAuthFlow: OAuthFlow;
  updateProfile: Scalars['Boolean']['output'];
};


export type MutationAddDataSourceArgs = {
  name: Scalars['String']['input'];
  path: Scalars['String']['input'];
};


export type MutationCompleteOAuthFlowArgs = {
  authCode: Scalars['String']['input'];
  state: Scalars['String']['input'];
};


export type MutationConnectMcpServerArgs = {
  input: ConnectMcpServerInput;
};


export type MutationCreateChatArgs = {
  name: Scalars['String']['input'];
};


export type MutationDeleteChatArgs = {
  chatId: Scalars['ID']['input'];
};


export type MutationDeleteDataSourceArgs = {
  id: Scalars['ID']['input'];
};


export type MutationSendMessageArgs = {
  chatId: Scalars['ID']['input'];
  text: Scalars['String']['input'];
};


export type MutationSendTelegramMessageArgs = {
  chatUUID: Scalars['ID']['input'];
  text: Scalars['String']['input'];
};


export type MutationStartOAuthFlowArgs = {
  provider: Scalars['String']['input'];
  scope: Scalars['String']['input'];
};


export type MutationUpdateProfileArgs = {
  input: UpdateProfileInput;
};

export type OAuthFlow = {
  __typename?: 'OAuthFlow';
  authURL: Scalars['String']['output'];
  redirectURI: Scalars['String']['output'];
};

export type OAuthStatus = {
  __typename?: 'OAuthStatus';
  expiresAt: Scalars['DateTime']['output'];
  provider: Scalars['String']['output'];
  scope: Array<Scalars['String']['output']>;
};

export type Query = {
  __typename?: 'Query';
  getChat: Chat;
  getChatSuggestions: Array<ChatSuggestionsCategory>;
  getChats: Array<Chat>;
  getDataSources: Array<DataSource>;
  getMCPServers: Array<McpServerDefinition>;
  getOAuthStatus: Array<OAuthStatus>;
  getTools: Array<Tool>;
  profile: UserProfile;
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

export enum Role {
  Assistant = 'ASSISTANT',
  User = 'USER'
}

export type Subscription = {
  __typename?: 'Subscription';
  indexingStatus: IndexingStatus;
  messageAdded: Message;
  telegramMessageAdded: Message;
  toolCallUpdated: ToolCall;
};


export type SubscriptionMessageAddedArgs = {
  chatId: Scalars['ID']['input'];
};


export type SubscriptionTelegramMessageAddedArgs = {
  chatUUID: Scalars['ID']['input'];
};


export type SubscriptionToolCallUpdatedArgs = {
  chatId: Scalars['ID']['input'];
};

export type Tool = {
  __typename?: 'Tool';
  description: Scalars['String']['output'];
  name: Scalars['String']['output'];
};

export type ToolCall = {
  __typename?: 'ToolCall';
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

export type UpdateProfileInput = {
  bio?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
};

export type UserProfile = {
  __typename?: 'UserProfile';
  bio?: Maybe<Scalars['String']['output']>;
  connectedDataSources: Array<DataSource>;
  indexingStatus?: Maybe<IndexingStatus>;
  name?: Maybe<Scalars['String']['output']>;
};

export type GetProfileQueryVariables = Exact<{ [key: string]: never; }>;


export type GetProfileQuery = { __typename?: 'Query', profile: { __typename?: 'UserProfile', name?: string | null } };

export type GetChatsQueryVariables = Exact<{
  first: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
}>;


export type GetChatsQuery = { __typename?: 'Query', getChats: Array<{ __typename?: 'Chat', id: string, name: string, createdAt: any, messages: Array<{ __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string }> }> }> };

export type GetChatQueryVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type GetChatQuery = { __typename?: 'Query', getChat: { __typename?: 'Chat', id: string, name: string, createdAt: any, messages: Array<{ __typename?: 'Message', id: string, text?: string | null, imageUrls: Array<string>, role: Role, createdAt: any, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string }> }> } };

export type GetDataSourcesQueryVariables = Exact<{ [key: string]: never; }>;


export type GetDataSourcesQuery = { __typename?: 'Query', getDataSources: Array<{ __typename?: 'DataSource', id: string, name: string, path: string, updatedAt: any, isProcessed: boolean, isIndexed: boolean, hasError: boolean }> };

export type GetOAuthStatusQueryVariables = Exact<{ [key: string]: never; }>;


export type GetOAuthStatusQuery = { __typename?: 'Query', getOAuthStatus: Array<{ __typename?: 'OAuthStatus', provider: string, expiresAt: any, scope: Array<string> }> };

export type GetChatSuggestionsQueryVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type GetChatSuggestionsQuery = { __typename?: 'Query', getChatSuggestions: Array<{ __typename?: 'ChatSuggestionsCategory', category: string, suggestions: Array<string> }> };

export type GetMcpServersQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMcpServersQuery = { __typename?: 'Query', getMCPServers: Array<{ __typename?: 'MCPServerDefinition', id: string, name: string, command: string, args?: Array<string> | null, type: McpServerType, enabled: boolean, connected: boolean, envs?: Array<{ __typename?: 'KeyValue', key: string, value: string }> | null }> };

export type CreateChatMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type CreateChatMutation = { __typename?: 'Mutation', createChat: { __typename?: 'Chat', id: string, name: string, createdAt: any } };

export type SendMessageMutationVariables = Exact<{
  chatId: Scalars['ID']['input'];
  text: Scalars['String']['input'];
}>;


export type SendMessageMutation = { __typename?: 'Mutation', sendMessage: { __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean }> } };

export type DeleteChatMutationVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type DeleteChatMutation = { __typename?: 'Mutation', deleteChat: { __typename?: 'Chat', id: string, name: string } };

export type MessageAddedSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type MessageAddedSubscription = { __typename?: 'Subscription', messageAdded: { __typename?: 'Message', id: string, text?: string | null, role: Role, createdAt: any, imageUrls: Array<string>, toolResults: Array<string>, toolCalls: Array<{ __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string }> } };

export type ToolCallUpdatedSubscriptionVariables = Exact<{
  chatId: Scalars['ID']['input'];
}>;


export type ToolCallUpdatedSubscription = { __typename?: 'Subscription', toolCallUpdated: { __typename?: 'ToolCall', id: string, name: string, isCompleted: boolean, messageId: string } };

export type IndexingStatusSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type IndexingStatusSubscription = { __typename?: 'Subscription', indexingStatus: { __typename?: 'IndexingStatus', status: IndexingState, error?: string | null, dataSources: Array<{ __typename?: 'DataSource', id: string, name: string, isProcessed: boolean, isIndexed: boolean, indexProgress: number, hasError: boolean }> } };

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


export const GetProfileDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetProfile"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"profile"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<GetProfileQuery, GetProfileQueryVariables>;
export const GetChatsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChats"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"first"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"offset"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChats"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"first"},"value":{"kind":"Variable","name":{"kind":"Name","value":"first"}}},{"kind":"Argument","name":{"kind":"Name","value":"offset"},"value":{"kind":"Variable","name":{"kind":"Name","value":"offset"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}}]}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}}]}}]}}]}}]} as unknown as DocumentNode<GetChatsQuery, GetChatsQueryVariables>;
export const GetChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}}]}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}}]}}]}}]}}]} as unknown as DocumentNode<GetChatQuery, GetChatQueryVariables>;
export const GetDataSourcesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetDataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getDataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"isProcessed"}},{"kind":"Field","name":{"kind":"Name","value":"isIndexed"}},{"kind":"Field","name":{"kind":"Name","value":"hasError"}}]}}]}}]} as unknown as DocumentNode<GetDataSourcesQuery, GetDataSourcesQueryVariables>;
export const GetOAuthStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetOAuthStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getOAuthStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"provider"}},{"kind":"Field","name":{"kind":"Name","value":"expiresAt"}},{"kind":"Field","name":{"kind":"Name","value":"scope"}}]}}]}}]} as unknown as DocumentNode<GetOAuthStatusQuery, GetOAuthStatusQueryVariables>;
export const GetChatSuggestionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetChatSuggestions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getChatSuggestions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"category"}},{"kind":"Field","name":{"kind":"Name","value":"suggestions"}}]}}]}}]} as unknown as DocumentNode<GetChatSuggestionsQuery, GetChatSuggestionsQueryVariables>;
export const GetMcpServersDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"GetMCPServers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"getMCPServers"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"command"}},{"kind":"Field","name":{"kind":"Name","value":"args"}},{"kind":"Field","name":{"kind":"Name","value":"envs"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"key"}},{"kind":"Field","name":{"kind":"Name","value":"value"}}]}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"enabled"}},{"kind":"Field","name":{"kind":"Name","value":"connected"}}]}}]}}]} as unknown as DocumentNode<GetMcpServersQuery, GetMcpServersQueryVariables>;
export const CreateChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}}]}}]}}]} as unknown as DocumentNode<CreateChatMutation, CreateChatMutationVariables>;
export const SendMessageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SendMessage"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"text"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"sendMessage"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}},{"kind":"Argument","name":{"kind":"Name","value":"text"},"value":{"kind":"Variable","name":{"kind":"Name","value":"text"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}}]}}]}}]}}]} as unknown as DocumentNode<SendMessageMutation, SendMessageMutationVariables>;
export const DeleteChatDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteChat"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteChat"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}}]}}]}}]} as unknown as DocumentNode<DeleteChatMutation, DeleteChatMutationVariables>;
export const MessageAddedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"MessageAdded"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"messageAdded"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"text"}},{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"imageUrls"}},{"kind":"Field","name":{"kind":"Name","value":"toolResults"}},{"kind":"Field","name":{"kind":"Name","value":"toolCalls"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}}]}}]}}]}}]} as unknown as DocumentNode<MessageAddedSubscription, MessageAddedSubscriptionVariables>;
export const ToolCallUpdatedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"ToolCallUpdated"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"toolCallUpdated"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"chatId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"chatId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isCompleted"}},{"kind":"Field","name":{"kind":"Name","value":"messageId"}}]}}]}}]} as unknown as DocumentNode<ToolCallUpdatedSubscription, ToolCallUpdatedSubscriptionVariables>;
export const IndexingStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"IndexingStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"indexingStatus"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"error"}},{"kind":"Field","name":{"kind":"Name","value":"dataSources"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"isProcessed"}},{"kind":"Field","name":{"kind":"Name","value":"isIndexed"}},{"kind":"Field","name":{"kind":"Name","value":"indexProgress"}},{"kind":"Field","name":{"kind":"Name","value":"hasError"}}]}}]}}]}}]} as unknown as DocumentNode<IndexingStatusSubscription, IndexingStatusSubscriptionVariables>;
export const AddDataSourceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"AddDataSource"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"addDataSource"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}]}]}}]} as unknown as DocumentNode<AddDataSourceMutation, AddDataSourceMutationVariables>;
export const DeleteDataSourceDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteDataSource"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteDataSource"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteDataSourceMutation, DeleteDataSourceMutationVariables>;
export const StartOAuthFlowDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StartOAuthFlow"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"provider"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"scope"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"startOAuthFlow"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"provider"},"value":{"kind":"Variable","name":{"kind":"Name","value":"provider"}}},{"kind":"Argument","name":{"kind":"Name","value":"scope"},"value":{"kind":"Variable","name":{"kind":"Name","value":"scope"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"authURL"}},{"kind":"Field","name":{"kind":"Name","value":"redirectURI"}}]}}]}}]} as unknown as DocumentNode<StartOAuthFlowMutation, StartOAuthFlowMutationVariables>;
export const CompleteOAuthFlowDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CompleteOAuthFlow"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"state"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"authCode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"completeOAuthFlow"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"state"},"value":{"kind":"Variable","name":{"kind":"Name","value":"state"}}},{"kind":"Argument","name":{"kind":"Name","value":"authCode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"authCode"}}}]}]}}]} as unknown as DocumentNode<CompleteOAuthFlowMutation, CompleteOAuthFlowMutationVariables>;