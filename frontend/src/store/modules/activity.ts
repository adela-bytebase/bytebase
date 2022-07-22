import { defineStore } from "pinia";
import axios from "axios";
import { stringify } from "qs";
import {
  Activity,
  ActivityCreate,
  ActivityId,
  ActivityPatch,
  ActivityState,
  IssueId,
  PrincipalId,
  ProjectId,
  ResourceObject,
} from "@/types";
import { useAuthStore } from "./auth";
import { getPrincipalFromIncludedList } from "./principal";

function convert(
  activity: ResourceObject,
  includedList: ResourceObject[]
): Activity {
  const payload = activity.attributes.payload
    ? JSON.parse((activity.attributes.payload as string) || "{}")
    : {};
  return {
    ...(activity.attributes as Omit<Activity, "id" | "creator" | "updater">),
    creator: getPrincipalFromIncludedList(
      activity.relationships!.creator.data,
      includedList
    ),
    updater: getPrincipalFromIncludedList(
      activity.relationships!.updater.data,
      includedList
    ),
    id: parseInt(activity.id),
    payload,
  };
}

export const useActivityStore = defineStore("activity", {
  state: (): ActivityState => ({
    activityListByUser: new Map(),
    activityListByIssue: new Map(),
  }),
  actions: {
    convert(
      activity: ResourceObject,
      includedList: ResourceObject[]
    ): Activity {
      return convert(activity, includedList || []);
    },
    getActivityListByUser(userId: PrincipalId): Activity[] {
      return this.activityListByUser.get(userId) || [];
    },
    getActivityListByIssue(issueId: IssueId): Activity[] {
      return this.activityListByIssue.get(issueId) || [];
    },
    setActivityListForUser({
      userId,
      activityList,
    }: {
      userId: PrincipalId;
      activityList: Activity[];
    }) {
      this.activityListByUser.set(userId, activityList);
    },
    setActivityListForIssue({
      issueId,
      activityList,
    }: {
      issueId: IssueId;
      activityList: Activity[];
    }) {
      this.activityListByIssue.set(issueId, activityList);
    },
    async fetchActivityListForUser(userId: PrincipalId) {
      const data = (await axios.get(`/api/activity?order=DESC`)).data;
      const activityList: Activity[] = data.data.map(
        (activity: ResourceObject) => {
          return convert(activity, data.included);
        }
      );

      this.setActivityListForUser({ userId, activityList });
      return activityList;
    },
    async fetchActivityList(params: {
      typePrefix: string;
      container: number | string;
      order: "ASC" | "DESC";
      limit?: number;
    }) {
      const url = `/api/activity?${stringify(params)}`;
      const response = (await axios.get(url)).data;
      const activityList: Activity[] = response.data.map(
        (activity: ResourceObject) => {
          return convert(activity, response.included);
        }
      );
      return activityList;
    },
    async fetchActivityListForIssue(issueId: IssueId) {
      const requestListForIssue = this.fetchActivityList({
        typePrefix: "bb.issue.",
        container: issueId,
        order: "ASC",
      });
      const requestListForPipeline = this.fetchActivityList({
        typePrefix: "bb.pipeline.",
        container: issueId,
        order: "ASC",
      });
      const [listForIssue, listForPipeline] = await Promise.all([
        requestListForIssue,
        requestListForPipeline,
      ]);

      const mergedList = [...listForIssue, ...listForPipeline];
      mergedList.sort((a, b) => {
        if (a.createdTs !== b.createdTs) {
          return a.createdTs - b.createdTs;
        }

        return a.id - b.id;
      });

      this.setActivityListForIssue({ issueId, activityList: mergedList });
      return mergedList;
    },
    // We do not store the returned list because the caller will specify different limits
    async fetchActivityListForProject({
      projectId,
      limit,
    }: {
      projectId: ProjectId;
      limit?: number;
    }) {
      const queryList = [
        "typePrefix=bb.project.",
        `container=${projectId}`,
        `order=DESC`,
      ];
      if (limit) {
        queryList.push(`limit=${limit}`);
      }
      const data = (await axios.get(`/api/activity?${queryList.join("&")}`))
        .data;
      const activityList: Activity[] = data.data.map(
        (activity: ResourceObject) => {
          return convert(activity, data.included);
        }
      );

      return activityList;
    },
    async fetchActivityListForQueryHistory({ limit }: { limit: number }) {
      const { currentUser } = useAuthStore();
      const queryList = [
        "typePrefix=bb.sql-editor.query",
        `user=${currentUser.id}`,
        `order=DESC`,
        `limit=${limit}`,
        // only fetch the successful query history
        `level=INFO`,
      ];
      const data = (await axios.get(`/api/activity?${queryList.join("&")}`))
        .data;
      const activityList: Activity[] = data.data.map(
        (activity: ResourceObject) => {
          return convert(activity, data.included);
        }
      );

      return activityList;
    },
    async fetchActivityListForDatabaseByProjectId({
      projectId,
      limit,
    }: {
      projectId: ProjectId;
      limit?: number;
    }) {
      const queryList = [
        "typePrefix=bb.database.",
        `container=${projectId}`,
        `order=DESC`,
      ];
      if (limit) {
        queryList.push(`limit=${limit}`);
      }
      const data = (await axios.get(`/api/activity?${queryList.join("&")}`))
        .data;
      const activityList: Activity[] = data.data.map(
        (activity: ResourceObject) => {
          return convert(activity, data.included);
        }
      );

      return activityList;
    },
    async createActivity(newActivity: ActivityCreate) {
      const data = (
        await axios.post(`/api/activity`, {
          data: {
            type: "activityCreate",
            attributes: newActivity,
          },
        })
      ).data;
      const createdActivity = convert(data.data, data.included);

      // There might exist other activities happened since the last fetch, so we do a full refetch.
      if (newActivity.type.startsWith("bb.issue.")) {
        this.fetchActivityListForIssue(newActivity.containerId);
      }

      return createdActivity;
    },
    async updateComment({
      activityId,
      updatedComment,
    }: {
      activityId: ActivityId;
      updatedComment: string;
    }) {
      const activityPatch: ActivityPatch = {
        comment: updatedComment,
      };
      const data = (
        await axios.patch(`/api/activity/${activityId}`, {
          data: {
            type: "activityPatch",
            attributes: activityPatch,
          },
        })
      ).data;
      const updatedActivity = convert(data.data, data.included);

      this.fetchActivityListForIssue(updatedActivity.containerId);

      return updatedActivity;
    },
    async deleteActivity(activity: Activity) {
      await axios.delete(`/api/activity/${activity.id}`);

      if (activity.type.startsWith("bb.issue.")) {
        this.fetchActivityListForIssue(activity.containerId);
      }
    },
    async deleteActivityById(id: number) {
      await axios.delete(`/api/activity/${id}`);
    },
  },
});
