import { homeRoute } from "./home";
import { notFoundRoute } from "./not-found";
import { relayOpsRoute } from "./ops-relay";
import { rootRoute } from "./root";

export const routeTree = rootRoute.addChildren([homeRoute, relayOpsRoute, notFoundRoute]);
