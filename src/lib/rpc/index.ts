/**
 * WS RPC layer — generic protobuf RPC over WebSocket.
 *
 * Each namespace keeps the same export API as the previous Connect RPC layer
 * so page components don't need to change.
 *
 * Auth (login/register/refresh/logout) stays in api.ts as REST.
 * Upload (accountApi.uploadAvatar) stays in api.ts (binary FormData).
 * Game sessions stay as raw WebSocket to /ws/v1/.
 */

import { toBinary, fromBinary } from "@bufbuild/protobuf";
import { getWSRpcTransport } from "@/lib/ws-rpc-transport";
import { toPlain } from "./helpers";

// ── Service name constants ─────────────────────────────────────────────
const SHOP = "rockgame.shop.ShopService";
const GAME_MANAGER = "rockgame.gamemanager.GameManagerService";
const LOBBY_SESSION = "rockgame.lobbysession.LobbySessionService";
const ACCOUNT = "rockgame.account.AccountService";
const ACTIVITY = "rockgame.activity.ActivityService";
const USER = "rockgame.user.UserService";
const AGENT = "rockgame.agent.AgentService";
const RANK = "rockgame.rank.RankService";
const VIP = "rockgame.user.UserService";

// ── Message types ─────────────────────────────────────────────────────
import {
  GetShopWalletRequest, ShopWalletResponse,
  GetPaymentChannelsRequest, PaymentChannelsResponse,
  GetWithdrawChannelsRequest, WithdrawChannelsResponse,
  CreateRechargeRequest, CreateRechargeResponse,
  CreateWithdrawRequest, CreateWithdrawResponse,
  GetOrdersRequest, OrdersResponse,
  GetPaymentAccountsRequest, PaymentAccountsResponse,
  SavePaymentAccountRequest, SavePaymentAccountResponse,
  SetWithdrawPasswordRequest, SetWithdrawPasswordResponse,
  GetPaymentMethodsRequest, PaymentMethodsResponse,
  GetWithdrawMethodsRequest, PaymentMethodsResponse,
  GetDepositAmountOptionsRequest, AmountOptionsResponse,
  GetWithdrawAmountOptionsRequest, AmountOptionsResponse as WithdrawAmountOptionsResponse,
  GetDepositProductsRequest, ShopProductsResponse,
  GetWithdrawProductsRequest, ShopProductsResponse as WithdrawProductsResponse,
} from "@/proto/shop_pb";
// ── Message types (GameManagerService — data queries) ──────────────────
import {
  GetLobbyBannersRequest, LobbyBannersResponse,
  GetLobbyCategoriesRequest, LobbyCategoriesResponse,
  GetLobbyGamesRequest, LobbyGamesResponse,
  GetLobbyConfigRequest, LobbyConfigResponse,
  GetLobbySplashRequest, LobbySplashResponse,
  GetGameVendorsRequest, GameVendorsResponse,
  SearchGamesRequest, SearchGamesResponse,
} from "@/proto/game_manager_pb";
// ── Message types (LobbySessionService — session + user state) ────────────────
import {
  GetRecentGamesRequest, RecentGamesResponse,
  ToggleFavoriteRequest, ToggleFavoriteResponse,
  GetGameHistoryRequest, GameHistoryResponse,
  EndGameSessionRequest, EndGameSessionResponse,
  LaunchSelfGameRequest, LaunchSelfGameResponse,
  GetReddotStateRequest, ReddotStateResponse,
  MarkReddotReadRequest, MarkReddotReadResponse,
} from "@/proto/lobby_session_pb";
import {
  GetProfileRequest, GetProfileResponse,
  GetAssetsRequest, GetAssetsResponse,
  UpdateProfileRequest, UpdateProfileResponse,
  ChangePasswordRequest, ChangePasswordResponse,
  DeleteAccountRequest, DeleteAccountResponse,
} from "@/proto/account_pb";
import {
  ListActivitiesRequest, ActivitiesResponse,
  CheckInRequest, CheckInResponse,
  GetCheckInStateRequest, CheckInStateResponse,
  GetCheckInConfigRequest, CheckInConfigResponse,
  ClaimRechargeBonusRequest, ClaimRechargeBonusResponse as ActivityClaimRechargeBonusResponse,
  ClaimTimedGiftRequest, ClaimTimedGiftResponse,
  GetTimedGiftStatusRequest, TimedGiftStatusResponse,
  SpinWheelRequest, SpinWheelResponse,
  GetWheelStateRequest, WheelStateResponse,
  GetWheelConfigRequest, WheelConfigResponse,
} from "@/proto/activity_pb";
import {
  GetVipLevelsRequest, VipLevelsResponse,
  GetVipInfoRequest, VipInfoResponse,
  GetInventoryRequest, InventoryResponse,
  GetItemDefineListRequest, ItemDefineListResponse,
  UseItemRequest, UseItemResponse,
  TransferItemRequest, TransferItemResponse,
  GetDailyTasksRequest, DailyTasksResponse,
  GetWeeklyTasksRequest, WeeklyTasksResponse,
  GetGrowthTasksRequest, GrowthTasksResponse,
  GetTaskProgressRequest, TaskProgressResponse,
  ClaimTaskRequest, ClaimTaskResponse,
  GetInboxRequest, InboxResponse,
  ReadMailRequest, ReadMailResponse,
  ClaimAttachmentRequest, ClaimAttachmentResponse,
  GetUnreadCountRequest, UnreadCountResponse,
  DeleteMailRequest, DeleteMailResponse,
} from "@/proto/user_pb";
import {
  GetAgentInfoRequest, AgentInfoResponse,
  GetSubordinatesRequest, SubordinatesResponse,
  GetCommissionsRequest, CommissionsResponse,
  RequestSettlementRequest, RequestSettlementResponse,
  CreatePromoLinkRequest, CreatePromoLinkResponse,
  GetAgentDashboardRequest, AgentDashboardResponse,
} from "@/proto/agent_pb";
import {
  GetRankListRequest, RankListResponse,
  GetMyRankRequest, MyRankResponse,
  GetTopRanksRequest, TopRanksResponse,
} from "@/proto/rank_pb";
import type { Message, MessageType } from "@bufbuild/protobuf";

// ── Typed invoke helper ─────────────────────────────────────────────────

function rpc<T extends Message>(
  service: string,
  method: string,
  req: Message,
  RespType: MessageType<T>,
): Promise<ReturnType<typeof toPlain>> {
  return getWSRpcTransport().invoke(service, method, req, RespType).then(toPlain);
}

// ═══════════════════════════════════════════════════════════════════════
// Shop RPCs  (rockgame.shop.ShopService)
// ═══════════════════════════════════════════════════════════════════════

export const shopRpc = {
  getWallet: () =>
    rpc(SHOP, "GetShopWallet", new GetShopWalletRequest(), ShopWalletResponse),

  getPaymentChannels: () =>
    rpc(SHOP, "GetPaymentChannels", new GetPaymentChannelsRequest(), PaymentChannelsResponse),

  getWithdrawChannels: () =>
    rpc(SHOP, "GetWithdrawChannels", new GetWithdrawChannelsRequest(), WithdrawChannelsResponse),

  getPaymentMethods: () =>
    rpc(SHOP, "GetPaymentMethods", new GetPaymentMethodsRequest(), PaymentMethodsResponse),

  getWithdrawMethods: () =>
    rpc(SHOP, "GetWithdrawMethods", new GetWithdrawMethodsRequest(), PaymentMethodsResponse),

  recharge: (data: { channel_id: number; product_id?: number; amount?: number }) =>
    rpc(SHOP, "CreateRecharge",
      new CreateRechargeRequest({
        channelId: BigInt(data.channel_id),
        productId: BigInt(data.product_id ?? 0),
        amount: BigInt(data.amount ?? 0),
      }), CreateRechargeResponse),

  withdraw: (data: { channel_id: number; amount: number; account?: string; account_name?: string; withdraw_password?: string }) =>
    rpc(SHOP, "CreateWithdraw",
      new CreateWithdrawRequest({
        channelId: BigInt(data.channel_id),
        amount: BigInt(data.amount),
        accountInfo: data.account ?? "",
        withdrawPassword: data.withdraw_password ?? "",
      }), CreateWithdrawResponse),

  getOrders: (params?: { type?: string; page?: number; page_size?: number }) =>
    rpc(SHOP, "GetOrders",
      new GetOrdersRequest({
        type: params?.type ?? "",
        page: params?.page ?? 1,
        pageSize: params?.page_size ?? 20,
      }), OrdersResponse),

  getPaymentAccounts: () =>
    rpc(SHOP, "GetPaymentAccounts", new GetPaymentAccountsRequest(), PaymentAccountsResponse),

  setPaymentAccount: (data: { id?: number; account_type: number; title: string; account: string; code?: string; username?: string }) =>
    rpc(SHOP, "SavePaymentAccount",
      new SavePaymentAccountRequest({
        bankName: data.title,
        accountNumber: data.account,
        accountName: data.username ?? "",
        type: String(data.account_type),
      }), SavePaymentAccountResponse),

  setWithdrawPassword: (data: { old_pwd?: string; new_pwd: string }) =>
    rpc(SHOP, "SetWithdrawPassword",
      new SetWithdrawPasswordRequest({
        password: data.old_pwd ?? "",
        newPassword: data.new_pwd,
      }), SetWithdrawPasswordResponse),

  getWithdrawAmountOptions: () =>
    rpc(SHOP, "GetWithdrawAmountOptions", new GetWithdrawAmountOptionsRequest(), WithdrawAmountOptionsResponse),

  getDepositAmountOptions: () =>
    rpc(SHOP, "GetDepositAmountOptions", new GetDepositAmountOptionsRequest(), AmountOptionsResponse),

  getDepositProducts: () =>
    rpc(SHOP, "GetDepositProducts", new GetDepositProductsRequest(), ShopProductsResponse),

  getWithdrawProducts: () =>
    rpc(SHOP, "GetWithdrawProducts", new GetWithdrawProductsRequest(), WithdrawProductsResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Game Manager RPCs  (rockgame.gamemanager.GameManagerService)
// ═══════════════════════════════════════════════════════════════════════

export const lobbyRpc = {
  getBanners: () =>
    rpc(GAME_MANAGER, "GetLobbyBanners", new GetLobbyBannersRequest(), LobbyBannersResponse),

  getCategories: () =>
    rpc(GAME_MANAGER, "GetLobbyCategories", new GetLobbyCategoriesRequest(), LobbyCategoriesResponse),

  getGames: (params?: { category_id?: number; vendor_id?: number; keyword?: string; page?: number; page_size?: number }) =>
    rpc(GAME_MANAGER, "GetLobbyGames",
      new GetLobbyGamesRequest({
        categoryId: String(params?.category_id ?? ""),
        vendorId: String(params?.vendor_id ?? ""),
        keyword: params?.keyword ?? "",
        page: params?.page ?? 1,
        pageSize: params?.page_size ?? 20,
      }), LobbyGamesResponse),

  getConfig: () =>
    rpc(GAME_MANAGER, "GetLobbyConfig", new GetLobbyConfigRequest(), LobbyConfigResponse),

  getSplash: () =>
    rpc(GAME_MANAGER, "GetLobbySplash", new GetLobbySplashRequest(), LobbySplashResponse),

  getVendors: () =>
    rpc(GAME_MANAGER, "GetGameVendors", new GetGameVendorsRequest(), GameVendorsResponse),

  searchGames: (keyword: string, page?: number, pageSize?: number) =>
    rpc(GAME_MANAGER, "SearchGames",
      new SearchGamesRequest({ keyword, limit: pageSize ?? 20 }), SearchGamesResponse),

  // User-state RPCs (LobbySessionService)
  getRecentGames: () =>
    rpc(LOBBY_SESSION, "GetRecentGames", new GetRecentGamesRequest(), RecentGamesResponse),

  endSession: (sessionId: string) =>
    rpc(LOBBY_SESSION, "EndGameSession",
      new EndGameSessionRequest({ sessionToken: sessionId }), EndGameSessionResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Game History RPCs  (rockgame.lobbysession.LobbySessionService)
// ═══════════════════════════════════════════════════════════════════════

export const historyRpc = {
  list: (params: { type?: string; page?: number; page_size?: number } = {}) =>
    rpc(LOBBY_SESSION, "GetGameHistory",
      new GetGameHistoryRequest({
        type: params.type ?? "",
        page: params.page ?? 1,
        pageSize: params.page_size ?? 20,
      }), GameHistoryResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Game Session RPCs  (rockgame.lobbysession.LobbySessionService)
// ═══════════════════════════════════════════════════════════════════════

export const gameRpc = {
  launch: (id: number) =>
    rpc(LOBBY_SESSION, "LaunchSelfGame",
      new LaunchSelfGameRequest({ id: BigInt(id) }), LaunchSelfGameResponse),

  toggleFavorite: (gameId: number) =>
    rpc(LOBBY_SESSION, "ToggleFavorite",
      new ToggleFavoriteRequest({ gameId: BigInt(gameId) }), ToggleFavoriteResponse),

  getRecentGames: () =>
    rpc(LOBBY_SESSION, "GetRecentGames", new GetRecentGamesRequest(), RecentGamesResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Account RPCs  (rockgame.account.AccountService)
// ═══════════════════════════════════════════════════════════════════════

export const accountRpc = {
  getProfile: () =>
    rpc(ACCOUNT, "GetProfile", new GetProfileRequest(), GetProfileResponse),

  getAssets: () =>
    rpc(ACCOUNT, "GetAssets", new GetAssetsRequest(), GetAssetsResponse),

  updateProfile: (data: { nickname?: string; avatar?: string; language?: string; timezone?: string }) =>
    rpc(ACCOUNT, "UpdateProfile", data as any, UpdateProfileResponse),

  changePassword: (data: { old_password: string; new_password: string }) =>
    rpc(ACCOUNT, "ChangePassword",
      new ChangePasswordRequest({
        oldPassword: data.old_password,
        newPassword: data.new_password,
        confirmPassword: data.new_password,
      }), ChangePasswordResponse),

  deleteAccount: () =>
    rpc(ACCOUNT, "DeleteAccount", new DeleteAccountRequest(), DeleteAccountResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Activity RPCs  (rockgame.activity.ActivityService)
// ═══════════════════════════════════════════════════════════════════════

export const activityRpc = {
  getList: () =>
    rpc(ACTIVITY, "ListActivities", new ListActivitiesRequest(), ActivitiesResponse),

  checkIn: () =>
    rpc(ACTIVITY, "CheckIn", new CheckInRequest(), CheckInResponse),

  getCheckInState: () =>
    rpc(ACTIVITY, "GetCheckInState", new GetCheckInStateRequest(), CheckInStateResponse),

  getCheckInConfig: () =>
    rpc(ACTIVITY, "GetCheckInConfig", new GetCheckInConfigRequest(), CheckInConfigResponse),

  claimRechargeBonus: () =>
    rpc(ACTIVITY, "ClaimRechargeBonus", new ClaimRechargeBonusRequest(), ActivityClaimRechargeBonusResponse),

  claimTimedGift: () =>
    rpc(ACTIVITY, "ClaimTimedGift", new ClaimTimedGiftRequest(), ClaimTimedGiftResponse),

  getTimedGiftStatus: () =>
    rpc(ACTIVITY, "GetTimedGiftStatus", new GetTimedGiftStatusRequest(), TimedGiftStatusResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// VIP RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const vipRpc = {
  getLevels: (lang?: string) =>
    rpc(USER, "GetVipLevels",
      new GetVipLevelsRequest({ lang: lang ?? "" }), VipLevelsResponse),

  getInfo: () =>
    rpc(USER, "GetVipInfo", new GetVipInfoRequest(), VipInfoResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Wheel / Lucky Spin RPCs  (rockgame.activity.ActivityService)
// ═══════════════════════════════════════════════════════════════════════

export const wheelRpc = {
  getConfig: () =>
    rpc(ACTIVITY, "GetWheelConfig", new GetWheelConfigRequest(), WheelConfigResponse),

  getState: () =>
    rpc(ACTIVITY, "GetWheelState", new GetWheelStateRequest(), WheelStateResponse),

  spin: () =>
    rpc(ACTIVITY, "SpinWheel", new SpinWheelRequest(), SpinWheelResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Item RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const itemRpc = {
  getInventory: () =>
    rpc(USER, "GetInventory", new GetInventoryRequest(), InventoryResponse),

  getList: () =>
    rpc(USER, "GetItemDefineList", new GetItemDefineListRequest(), ItemDefineListResponse),

  useItem: (data: { item_id: number; quantity?: number }) =>
    rpc(USER, "UseItem",
      new UseItemRequest({
        itemId: BigInt(data.item_id),
        quantity: data.quantity ?? 1,
      }), UseItemResponse),

  transfer: (data: { target_user_id: number; item_id: number; quantity: number }) =>
    rpc(USER, "TransferItem",
      new TransferItemRequest({
        toUserId: BigInt(data.target_user_id),
        itemId: BigInt(data.item_id),
        quantity: data.quantity,
      }), TransferItemResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Task RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const taskRpc = {
  getTaskConfig: async () => {
    const [daily, weekly, growth] = await Promise.allSettled([
      rpc(USER, "GetDailyTasks", new GetDailyTasksRequest(), DailyTasksResponse),
      rpc(USER, "GetWeeklyTasks", new GetWeeklyTasksRequest(), WeeklyTasksResponse),
      rpc(USER, "GetGrowthTasks", new GetGrowthTasksRequest(), GrowthTasksResponse),
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
    rpc(USER, "GetTaskProgress", new GetTaskProgressRequest(), TaskProgressResponse),

  claimReward: (taskId: number) =>
    rpc(USER, "ClaimTask",
      new ClaimTaskRequest({ taskId: BigInt(taskId) }), ClaimTaskResponse),

  claimAllRewards: async (taskIds: number[]) => {
    if (!taskIds.length) return { claimed_count: 0 };
    const results = await Promise.allSettled(
      taskIds.map((taskId) =>
        rpc(USER, "ClaimTask",
          new ClaimTaskRequest({ taskId: BigInt(taskId) }), ClaimTaskResponse),
      ),
    );
    const succeeded = results.filter((r) => r.status === "fulfilled").length;
    const failed = results.filter((r) => r.status === "rejected").length;
    if (failed > 0) throw new Error(`${succeeded} claimed, ${failed} failed`);
    return { claimed_count: succeeded };
  },
};

// ═══════════════════════════════════════════════════════════════════════
// Mail RPCs  (rockgame.user.UserService)
// ═══════════════════════════════════════════════════════════════════════

export const mailRpc = {
  getMailbox: () =>
    rpc(USER, "GetInbox", new GetInboxRequest(), InboxResponse),

  readMail: (id: number) =>
    rpc(USER, "ReadMail",
      new ReadMailRequest({ mailId: BigInt(id) }), ReadMailResponse),

  deleteMail: async (ids: number[]) => {
    const results = await Promise.allSettled(
      ids.map((id) =>
        rpc(USER, "DeleteMail",
          new DeleteMailRequest({ mailId: BigInt(id) }), DeleteMailResponse),
      ),
    );
    const failed = results.filter((r) => r.status === "rejected").length;
    if (failed > 0) throw new Error(`${failed} mail(s) failed to delete`);
    return { deleted_count: ids.length };
  },

  claimMailAttachment: (id: number) =>
    rpc(USER, "ClaimAttachment",
      new ClaimAttachmentRequest({ mailId: BigInt(id) }), ClaimAttachmentResponse),

  getUnreadCount: () =>
    rpc(USER, "GetUnreadCount", new GetUnreadCountRequest(), UnreadCountResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Rank RPCs  (rockgame.rank.RankService)
// ═══════════════════════════════════════════════════════════════════════

export const rankRpc = {
  getRankList: (rankType: string, period?: string, page?: number) =>
    rpc(RANK, "GetRankList",
      new GetRankListRequest({ type: rankType, period: period ?? "", page: page ?? 1 }), RankListResponse),

  getMyRank: (rankType: string) =>
    rpc(RANK, "GetMyRank",
      new GetMyRankRequest({ type: rankType }), MyRankResponse),

  getTopPlayers: (rankType: string, limit?: number) =>
    rpc(RANK, "GetTopRanks",
      new GetTopRanksRequest({ type: rankType, limit: limit ?? 10 }), TopRanksResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Agent RPCs  (rockgame.agent.AgentService)
// ═══════════════════════════════════════════════════════════════════════

export const agentRpc = {
  getAgentInfo: () =>
    rpc(AGENT, "GetAgentInfo", new GetAgentInfoRequest(), AgentInfoResponse),

  getSubordinates: () =>
    rpc(AGENT, "GetSubordinates", new GetSubordinatesRequest(), SubordinatesResponse),

  getCommissionRecords: () =>
    rpc(AGENT, "GetCommissions", new GetCommissionsRequest(), CommissionsResponse),

  getDashboard: () =>
    rpc(AGENT, "GetAgentDashboard", new GetAgentDashboardRequest(), AgentDashboardResponse),

  requestSettlement: () =>
    rpc(AGENT, "RequestSettlement", new RequestSettlementRequest(), RequestSettlementResponse),

  getPromoLink: () =>
    rpc(AGENT, "CreatePromoLink", new CreatePromoLinkRequest(), CreatePromoLinkResponse),
};

// ═══════════════════════════════════════════════════════════════════════
// Reddot RPCs  (rockgame.lobbysession.LobbySessionService)
// ═══════════════════════════════════════════════════════════════════════

export const reddotRpc = {
  getReddots: () =>
    rpc(LOBBY_SESSION, "GetReddotState", new GetReddotStateRequest(), ReddotStateResponse),

  markAsRead: (category: string) =>
    rpc(LOBBY_SESSION, "MarkReddotRead",
      new MarkReddotReadRequest({ category }), MarkReddotReadResponse),
};
