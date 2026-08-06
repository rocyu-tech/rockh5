/**
 * Connect/gRPC-Web RPC layer — drop-in replacement for axios API calls.
 *
 * Each namespace mirrors the shape in api.ts but calls via Connect protocol
 * through Gate's connectproxy. Returns plain objects (snake_case, number values)
 * so callers only need to change import path and remove `.data` accessor.
 *
 * Auth (login/register/refresh/logout) stays in api.ts as REST.
 * Upload (accountApi.uploadAvatar) stays in api.ts (binary FormData).
 */

import { createClient } from "@connectrpc/connect";
import { grpcTransport } from "@/lib/grpc-transport";
import { toPlain } from "./helpers";

// ── Service definitions ────────────────────────────────────────────────
import { ShopService } from "@/proto/shop_connect";
import { LobbyService } from "@/proto/lobby_connect";
import { AccountService } from "@/proto/account_connect";
import { ActivityService } from "@/proto/activity_connect";
import { UserService } from "@/proto/user_connect";
import { AgentService } from "@/proto/agent_connect";
import { RankService } from "@/proto/rank_connect";
import { VIPService } from "@/proto/vip_connect";

// ── Message types (requests) ─────────────────────────────────────────
import {
  GetShopWalletRequest,
  GetPaymentChannelsRequest,
  GetWithdrawChannelsRequest,
  CreateRechargeRequest,
  CreateWithdrawRequest,
  GetOrdersRequest,
  GetPaymentAccountsRequest,
  SavePaymentAccountRequest,
  SetWithdrawPasswordRequest,
  GetPaymentMethodsRequest,
  GetWithdrawMethodsRequest,
  GetDepositAmountOptionsRequest,
  GetWithdrawAmountOptionsRequest,
  GetDepositProductsRequest,
  GetWithdrawProductsRequest,
} from "@/proto/shop_pb";
import {
  GetLobbyBannersRequest,
  GetLobbyCategoriesRequest,
  GetLobbyGamesRequest,
  GetLobbyConfigRequest,
  GetLobbySplashRequest,
  GetGameVendorsRequest,
  GetRecentGamesRequest,
  SearchGamesRequest,
  ToggleFavoriteRequest,
  GetGameHistoryRequest,
  EndGameSessionRequest,
  GetReddotStateRequest,
  MarkReddotReadRequest,
} from "@/proto/lobby_pb";
import {
  GetProfileRequest,
  GetAssetsRequest,
  UpdateProfileRequest,
  ChangePasswordRequest,
  DeleteAccountRequest,
} from "@/proto/account_pb";
import {
  ListActivitiesRequest,
  CheckInRequest,
  GetCheckInStateRequest,
  GetCheckInConfigRequest,
  ClaimRechargeBonusRequest,
  ClaimTimedGiftRequest,
  GetTimedGiftStatusRequest,
  SpinWheelRequest,
  GetWheelStateRequest,
  GetWheelConfigRequest,
} from "@/proto/activity_pb";
import {
  GetVipLevelsRequest,
  GetVipInfoRequest,
  GetInventoryRequest,
  GetItemDefineListRequest,
  UseItemRequest,
  TransferItemRequest,
  GetDailyTasksRequest,
  GetWeeklyTasksRequest,
  GetGrowthTasksRequest,
  GetTaskProgressRequest,
  ClaimTaskRequest,
  GetInboxRequest,
  ReadMailRequest,
  ClaimAttachmentRequest,
  GetUnreadCountRequest,
  DeleteMailRequest,
} from "@/proto/user_pb";
import {
  GetAgentInfoRequest,
  GetSubordinatesRequest,
  GetCommissionsRequest,
  RequestSettlementRequest,
  CreatePromoLinkRequest,
  GetAgentDashboardRequest,
} from "@/proto/agent_pb";
import {
  GetRankListRequest,
  GetMyRankRequest,
  GetTopRanksRequest,
} from "@/proto/rank_pb";

// ── Clients ────────────────────────────────────────────────────────────
const shopClient = createClient(ShopService, grpcTransport);
const lobbyClient = createClient(LobbyService, grpcTransport);
const accountClient = createClient(AccountService, grpcTransport);
const activityClient = createClient(ActivityService, grpcTransport);
const userClient = createClient(UserService, grpcTransport);
const agentClient = createClient(AgentService, grpcTransport);
const rankClient = createClient(RankService, grpcTransport);
const vipClient = createClient(VIPService, grpcTransport);

// ═══════════════════════════════════════════════════════════════════════
// Shop RPCs  (rockgame.shop.ShopService)
// ═══════════════════════════════════════════════════════════════════════

export const shopRpc = {
  getWallet: () =>
    shopClient.getShopWallet(new GetShopWalletRequest()).then(toPlain),

  getPaymentChannels: () =>
    shopClient.getPaymentChannels(new GetPaymentChannelsRequest()).then(toPlain),

  getWithdrawChannels: () =>
    shopClient.getWithdrawChannels(new GetWithdrawChannelsRequest()).then(toPlain),

  getPaymentMethods: () =>
    shopClient.getPaymentMethods(new GetPaymentMethodsRequest()).then(toPlain),

  getWithdrawMethods: () =>
    shopClient.getWithdrawMethods(new GetWithdrawMethodsRequest()).then(toPlain),

  recharge: (data: { channel_id: number; product_id?: number; amount?: number }) =>
    shopClient.createRecharge(new CreateRechargeRequest({
      channelId: BigInt(data.channel_id),
      productId: BigInt(data.product_id ?? 0),
      amount: BigInt(data.amount ?? 0),
    })).then(toPlain),

  withdraw: (data: { channel_id: number; amount: number; account?: string; account_name?: string }) =>
    shopClient.createWithdraw(new CreateWithdrawRequest({
      channelId: BigInt(data.channel_id),
      amount: BigInt(data.amount),
      accountInfo: data.account ?? "",
      withdrawPassword: "",
    })).then(toPlain),

  getOrders: (params?: { type?: string; page?: number; page_size?: number }) =>
    shopClient.getOrders(new GetOrdersRequest({
      type: params?.type ?? "",
      page: params?.page ?? 1,
      pageSize: params?.page_size ?? 20,
    })).then(toPlain),

  getPaymentAccounts: () =>
    shopClient.getPaymentAccounts(new GetPaymentAccountsRequest()).then(toPlain),

  setPaymentAccount: (data: { id?: number; account_type: number; title: string; account: string; code?: string; username?: string }) =>
    shopClient.savePaymentAccount(new SavePaymentAccountRequest({
      bankName: data.title,
      accountNumber: data.account,
      accountName: data.username ?? "",
      type: String(data.account_type),
    })).then(toPlain),

  setWithdrawPassword: (data: { old_pwd?: string; new_pwd: string }) =>
    shopClient.setWithdrawPassword(new SetWithdrawPasswordRequest({
      password: data.old_pwd ?? "",
      newPassword: data.new_pwd,
    })).then(toPlain),

  getWithdrawAmountOptions: () =>
    shopClient.getWithdrawAmountOptions(new GetWithdrawAmountOptionsRequest()).then(toPlain),

  getDepositAmountOptions: () =>
    shopClient.getDepositAmountOptions(new GetDepositAmountOptionsRequest()).then(toPlain),

  getDepositProducts: () =>
    shopClient.getDepositProducts(new GetDepositProductsRequest()).then(toPlain),

  getWithdrawProducts: () =>
    shopClient.getWithdrawProducts(new GetWithdrawProductsRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Lobby RPCs  (rockgame.lobby.LobbyService)
// ═══════════════════════════════════════════════════════════════════════

export const lobbyRpc = {
  getBanners: () =>
    lobbyClient.getLobbyBanners(new GetLobbyBannersRequest()).then(toPlain),

  getCategories: () =>
    lobbyClient.getLobbyCategories(new GetLobbyCategoriesRequest()).then(toPlain),

  getGames: (params?: { category_id?: number; vendor_id?: number; keyword?: string; page?: number; page_size?: number }) =>
    lobbyClient.getLobbyGames({
      categoryId: String(params?.category_id ?? ""),
      vendorId: String(params?.vendor_id ?? ""),
      keyword: params?.keyword ?? "",
      page: params?.page ?? 1,
      pageSize: params?.page_size ?? 20,
    }).then(toPlain),

  getConfig: () =>
    lobbyClient.getLobbyConfig(new GetLobbyConfigRequest()).then(toPlain),

  getSplash: () =>
    lobbyClient.getLobbySplash(new GetLobbySplashRequest()).then(toPlain),

  getVendors: () =>
    lobbyClient.getGameVendors(new GetGameVendorsRequest()).then(toPlain),

  getRecentGames: () =>
    lobbyClient.getRecentGames(new GetRecentGamesRequest()).then(toPlain),

  searchGames: (keyword: string, page?: number, pageSize?: number) =>
    lobbyClient.searchGames(new SearchGamesRequest({
      keyword,
      limit: pageSize ?? 20,
    })).then(toPlain),

  toggleFavorite: (gameId: number) =>
    lobbyClient.toggleFavorite(new ToggleFavoriteRequest({
      gameId: BigInt(gameId),
    })).then(toPlain),

  endSession: (sessionId: string) =>
    lobbyClient.endGameSession(new EndGameSessionRequest({ sessionToken: sessionId })).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Game History RPCs  (rockgame.lobby.LobbyService)
// ═══════════════════════════════════════════════════════════════════════

export const historyRpc = {
  list: (params: { type?: string; page?: number; page_size?: number } = {}) =>
    lobbyClient.getGameHistory(new GetGameHistoryRequest({
      type: params.type ?? "",
      page: params.page ?? 1,
      pageSize: params.page_size ?? 20,
    })).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Game RPCs  (rockgame.lobby.LobbyService — game launch/favorites)
// ═══════════════════════════════════════════════════════════════════════

export const gameRpc = {
  launch: (id: number) =>
    lobbyClient.launchSelfGame({ id: BigInt(id) }).then(toPlain),

  toggleFavorite: (gameId: number) =>
    lobbyClient.toggleFavorite(new ToggleFavoriteRequest({
      gameId: BigInt(gameId),
    })).then(toPlain),

  getRecentGames: () =>
    lobbyClient.getRecentGames(new GetRecentGamesRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Account RPCs  (rockgame.account.AccountService)
// ═══════════════════════════════════════════════════════════════════════

export const accountRpc = {
  getProfile: () =>
    accountClient.getProfile(new GetProfileRequest()).then(toPlain),

  getAssets: () =>
    accountClient.getAssets(new GetAssetsRequest()).then(toPlain),

  updateProfile: (data: { nickname?: string; avatar?: string; language?: string; timezone?: string }) =>
    accountClient.updateProfile(data).then(toPlain),

  changePassword: (data: { old_password: string; new_password: string }) =>
    accountClient.changePassword(new ChangePasswordRequest({
      oldPassword: data.old_password,
      newPassword: data.new_password,
      confirmPassword: data.new_password,
    })).then(toPlain),

  deleteAccount: () =>
    accountClient.deleteAccount(new DeleteAccountRequest()).then(toPlain),

  // uploadAvatar stays in api.ts — requires multipart/form-data (not protobuf)
};

// ═══════════════════════════════════════════════════════════════════════
// Activity RPCs  (rockgame.activity.ActivityService)
// ═══════════════════════════════════════════════════════════════════════

export const activityRpc = {
  getList: () =>
    activityClient.listActivities(new ListActivitiesRequest()).then(toPlain),

  checkIn: () =>
    activityClient.checkIn(new CheckInRequest()).then(toPlain),

  getCheckInState: () =>
    activityClient.getCheckInState(new GetCheckInStateRequest()).then(toPlain),

  getCheckInConfig: () =>
    activityClient.getCheckInConfig(new GetCheckInConfigRequest()).then(toPlain),

  claimRechargeBonus: () =>
    activityClient.claimRechargeBonus(new ClaimRechargeBonusRequest()).then(toPlain),

  claimTimedGift: () =>
    activityClient.claimTimedGift(new ClaimTimedGiftRequest()).then(toPlain),

  getTimedGiftStatus: () =>
    activityClient.getTimedGiftStatus(new GetTimedGiftStatusRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// VIP RPCs  (rockgame.user.UserService — VIP methods)
// ═══════════════════════════════════════════════════════════════════════

export const vipRpc = {
  getLevels: (lang?: string) =>
    userClient.getVipLevels(new GetVipLevelsRequest({ lang: lang ?? "" })).then(toPlain),

  getInfo: () =>
    userClient.getVipInfo(new GetVipInfoRequest()).then(toPlain),

  upgrade: () =>
    // CheckVipUpgrade lives in VIPService
    vipClient.checkAndUpgrade({} as any).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Wheel / Lucky Spin RPCs  (rockgame.activity.ActivityService)
// ═══════════════════════════════════════════════════════════════════════

export const wheelRpc = {
  getConfig: () =>
    activityClient.getWheelConfig(new GetWheelConfigRequest()).then(toPlain),

  getState: () =>
    activityClient.getWheelState(new GetWheelStateRequest()).then(toPlain),

  spin: (useFree?: boolean) =>
    activityClient.spinWheel(new SpinWheelRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Item RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const itemRpc = {
  getInventory: () =>
    userClient.getInventory(new GetInventoryRequest()).then(toPlain),

  getList: () =>
    userClient.getItemDefineList(new GetItemDefineListRequest()).then(toPlain),

  useItem: (data: { item_id: number; quantity?: number }) =>
    userClient.useItem(new UseItemRequest({
      itemId: BigInt(data.item_id),
      quantity: data.quantity ?? 1,
    })).then(toPlain),

  transfer: (data: { target_user_id: number; item_id: number; quantity: number }) =>
    userClient.transferItem(new TransferItemRequest({
      toUserId: BigInt(data.target_user_id),
      itemId: BigInt(data.item_id),
      quantity: data.quantity,
    })).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Task RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const taskRpc = {
  getTaskConfig: async () => {
    const [daily, weekly, growth] = await Promise.allSettled([
      userClient.getDailyTasks(new GetDailyTasksRequest()).then(toPlain),
      userClient.getWeeklyTasks(new GetWeeklyTasksRequest()).then(toPlain),
      userClient.getGrowthTasks(new GetGrowthTasksRequest()).then(toPlain),
    ]);
    const getList = (r: PromiseSettledResult<Record<string, unknown>>): unknown[] =>
      r.status === "fulfilled" ? ((r.value as any)?.tasks || []) : [];
    const wrap = (list: unknown[], type: number) => ({
      task_type: type,
      receive_all_btn: list.some((t: any) => t.receive_status === 1) ? 1 : 0,
      task_type_state: list,
    });
    return [wrap(getList(daily), 0), wrap(getList(weekly), 1), wrap(getList(growth), 2)];
  },

  getTaskProgress: () =>
    userClient.getTaskProgress(new GetTaskProgressRequest()).then(toPlain),

  claimReward: (taskId: number) =>
    userClient.claimTask(new ClaimTaskRequest({
      taskId: BigInt(taskId),
    })).then(toPlain),

  claimAllRewards: (taskType?: number) =>
    userClient.claimTask(new ClaimTaskRequest({
      taskId: taskType ? BigInt(taskType) : undefined,
    })).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Mail RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const mailRpc = {
  getMailbox: () =>
    userClient.getInbox(new GetInboxRequest()).then(toPlain),

  readMail: (id: number) =>
    userClient.readMail(new ReadMailRequest({
      mailId: BigInt(id),
    })).then(toPlain),

  deleteMail: (ids: number[]) =>
    userClient.deleteMail(new DeleteMailRequest({
      mailId: ids.length > 0 ? BigInt(ids[0]) : undefined,
    })).then(toPlain),

  claimMailAttachment: (id: number) =>
    userClient.claimAttachment(new ClaimAttachmentRequest({
      mailId: BigInt(id),
    })).then(toPlain),

  getUnreadCount: () =>
    userClient.getUnreadCount(new GetUnreadCountRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Rank RPCs  (rockgame.rank.RankService)
// ═══════════════════════════════════════════════════════════════════════

export const rankRpc = {
  getRankList: (rankType: string, period?: string, page?: number) =>
    rankClient.getRankList(new GetRankListRequest({
      type: rankType,
      period: period ?? "",
      page: page ?? 1,
    })).then(toPlain),

  getMyRank: (rankType: string) =>
    rankClient.getMyRank(new GetMyRankRequest({ type: rankType })).then(toPlain),

  getTopPlayers: (rankType: string, limit?: number) =>
    rankClient.getTopRanks(new GetTopRanksRequest({
      type: rankType,
      limit: limit ?? 10,
    })).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Agent RPCs  (rockgame.agent.AgentService)
// ═══════════════════════════════════════════════════════════════════════

export const agentRpc = {
  getAgentInfo: () =>
    agentClient.getAgentInfo(new GetAgentInfoRequest()).then(toPlain),

  getSubordinates: (page?: number, pageSize?: number) =>
    agentClient.getSubordinates(new GetSubordinatesRequest()).then(toPlain),

  getCommissionRecords: (page?: number, pageSize?: number) =>
    agentClient.getCommissions(new GetCommissionsRequest()).then(toPlain),

  getDashboard: () =>
    agentClient.getAgentDashboard(new GetAgentDashboardRequest()).then(toPlain),

  requestSettlement: () =>
    agentClient.requestSettlement(new RequestSettlementRequest()).then(toPlain),

  getPromoLink: () =>
    agentClient.createPromoLink(new CreatePromoLinkRequest()).then(toPlain),
};

// ═══════════════════════════════════════════════════════════════════════
// Reddot RPCs  (rockgame.lobby.LobbyService)
// ═══════════════════════════════════════════════════════════════════════

export const reddotRpc = {
  getReddots: () =>
    lobbyClient.getReddotState(new GetReddotStateRequest()).then(toPlain),

  markAsRead: (category: string) =>
    lobbyClient.markReddotRead(new MarkReddotReadRequest({ category })).then(toPlain),
};
