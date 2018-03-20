package action

// AZURE
// instances := make([]*types.Instance, 0)
// groups, err := rgClient.List(ctx.Background(), "", nil)
// if err != nil {
// 	log.Errorf("[AZURE] Failed to fetch the existing resource groups, err: %s", err.Error())
// 	return nil, err
// }
// for _, g := range groups.Values() {
// 	resources, err := resClient.ListByResourceGroup(ctx.Background(), *g.Name, "", "", nil) // TODO maybe we can filter for (running) instances
// 	if err != nil {
// 		log.Warn("[AZURE] Failed to fetch the resources for %s, err: %s", *g.Name, err.Error())
// 		continue
// 	}
// 	for _, r := range resources.Values() {
// 		if _, ok := typesToCollect[*r.Type]; ok {
// 			if _, ok := r.Tags["Owner"]; !ok {
// 				instances = append(instances, &types.Instance{
// 					Name:      *r.Name,
// 					Id:        *r.ID,
// 					CloudType: types.AZURE,
// 					Tags:      utils.ConvertTags(r.Tags),
// 				})
// 			}
// 		}
// 	}
// }

// return instances, nil

// GCP
// instanceGroups, err := computeClient.InstanceGroupManagers.AggregatedList(projectId).Do()
// if err != nil {
// 	log.Errorf("[GCP] Failed to fetch instance groups, err: %s", err.Error())
// 	return nil, err
// }

// instancesToDelete := []*compute.Instance{}
// instanceGroupsToDelete := map[*compute.InstanceGroupManager]bool{}

// groupFound := false
// for _, i := range instanceGroups.Items {
// 	for _, group := range i.InstanceGroupManagers {
// 		if _, ok := instanceGroupsToDelete[group]; !ok && strings.Index(inst.Name, group.BaseInstanceName+"-") == 0 {
// 			instanceGroupsToDelete[group], groupFound = true, true
// 		}
// 	}
// }
// if !groupFound {
// instancesToDelete = append(instancesToDelete, inst)
// }

// for group, _ := range instanceGroupsToDelete {
// 	zone := getZone(group.Zone)
// 	log.Infof("[GCP] Deleting instance group %s in zone %s", group.Name, zone)
// 	if !context.DryRun {
// 		_, err := computeClient.InstanceGroupManagers.Delete(projectId, zone, group.Name).Do()
// 		if err != nil {
// 			log.Errorf("[GCP] Failed to delete instance group %s, err: %s", group.Name, err.Error())
// 		}
// 	}
// }
// for _, inst := range instancesToDelete {
// 	zone := getZone(inst.Zone)
// 	log.Infof("[GCP] Deleting instance %s in zone %s", inst.Name, zone)
// 	if !context.DryRun {
// 		_, err := computeClient.Instances.Delete(projectId, zone, inst.Name).Do()
// 		if err != nil {
// 			log.Errorf("[GCP] Failed to delete instance %s, err: %s", inst.Name, err.Error())
// 		}
// 	}
// }
