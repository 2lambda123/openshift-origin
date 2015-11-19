'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:DeploymentConfigController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('DeploymentConfigController', function ($scope, $routeParams, DataService, project, DeploymentsService, ImageStreamResolver, $filter, LabelFilter) {
    $scope.deploymentConfig = null;
    $scope.deployments = {};
    $scope.unfilteredDeployments = {};
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};         
    $scope.labelSuggestions = {};    
    // TODO we should add this back in and show the pod template on this page
    //$scope.podTemplates = {};
    //$scope.imageStreams = {};
    //$scope.imagesByDockerReference = {};
    //$scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    //$scope.builds = {};   
    $scope.alerts = {};
    $scope.breadcrumbs = [
      {
        title: "Deployments",
        link: "project/" + $routeParams.project + "/browse/deployments"
      },
      {
        title: $routeParams.deploymentconfig
      }
    ];
    $scope.emptyMessage = "Loading...";

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("deploymentconfigs", $routeParams.deploymentconfig, $scope).then(
        // success
        function(deploymentConfig) {
          $scope.loaded = true;
          $scope.deploymentConfig = deploymentConfig;
          ImageStreamResolver.fetchReferencedImageStreamImages([deploymentConfig.spec.template], $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("deploymentconfigs", $routeParams.deploymentconfig, $scope, function(deploymentConfig, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This deployment configuration has been deleted."
              }; 
            }
            $scope.deploymentConfig = deploymentConfig;
            ImageStreamResolver.fetchReferencedImageStreamImages([deploymentConfig.spec.template], $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
          }));          
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The deployment configuration details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      // TODO we should add this back in and show the pod template on this page
      // function extractPodTemplates() {
      //   angular.forEach($scope.deployments, function(deployment, deploymentId){
      //     $scope.podTemplates[deploymentId] = deployment.spec.template;
      //   });
      // }

      watches.push(DataService.watch("replicationcontrollers", $scope, function(deployments, action, deployment) { 
        // TODO we should add this back in and show the pod template on this page
        // extractPodTemplates();
        // ImageStreamResolver.fetchReferencedImageStreamImages($scope.podTemplates, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
        $scope.emptyMessage = "No deployments to show";
 
        if (!action) {
          var deploymentsByDeploymentConfig = DeploymentsService.associateDeploymentsToDeploymentConfig(deployments.by("metadata.name"));
          $scope.unfilteredDeployments = deploymentsByDeploymentConfig[$routeParams.deploymentconfig] || {};
          angular.forEach($scope.unfilteredDeployments, function(deployment) {
            deployment.causes = $filter('deploymentCauses')(deployment);
          });
          // Loading of the page that will create deploymentConfigDeploymentsInProgress structure, which will associate running deployment to his deploymentConfig.
          $scope.deploymentConfigDeploymentsInProgress = DeploymentsService.associateRunningDeploymentToDeploymentConfig(deploymentsByDeploymentConfig);
        } else if (DeploymentsService.deploymentBelongsToConfig(deployment, $routeParams.deploymentconfig)) {
          var deploymentName = deployment.metadata.name;
          var deploymentConfigName = $routeParams.deploymentconfig;
          switch (action) {
            case 'ADDED':
            case 'MODIFIED':
              $scope.unfilteredDeployments[deploymentName] = deployment;
              // When deployment is retried, associate him to his deploymentConfig and add him into deploymentConfigDeploymentsInProgress structure.
              if ($filter('deploymentIsInProgress')(deployment)){
                $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] = $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName] || {};
                $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName] = deployment;                
              } else if ($scope.deploymentConfigDeploymentsInProgress[deploymentConfigName]) { // After the deployment ends remove him from the deploymentConfigDeploymentsInProgress structure.
                delete $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName];
              }
              deployment.causes = $filter('deploymentCauses')(deployment);
              break;
            case 'DELETED':
              delete $scope.unfilteredDeployments[deploymentName];
              if ($scope.deploymentConfigDeploymentsInProgress[deploymentConfigName]) {
                delete $scope.deploymentConfigDeploymentsInProgress[deploymentConfigName][deploymentName];
              }
              break;
          }
        }

        $scope.deployments = LabelFilter.getLabelSelector().select($scope.unfilteredDeployments);      
        updateFilterWarning();
        LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredDeployments, $scope.labelSuggestions);
        LabelFilter.setLabelSuggestions($scope.labelSuggestions);
      }));

      watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
        $scope.imageStreams = imageStreams.by("metadata.name");
        ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
        // If the dep config has been loaded already
        if ($scope.deploymentConfig) {
          ImageStreamResolver.fetchReferencedImageStreamImages([$scope.deploymentConfig.spec.template], $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
        }
        Logger.log("imagestreams (subscribe)", $scope.imageStreams);
      }));

      watches.push(DataService.watch("builds", $scope, function(builds) {
        $scope.builds = builds.by("metadata.name");
        Logger.log("builds (subscribe)", $scope.builds);
      }));
    });

    function updateFilterWarning() {
      if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.deployments) && !$.isEmptyObject($scope.unfilteredDeployments)) {
        $scope.alerts["deployments"] = {
          type: "warning",
          details: "The active filters are hiding all deployments."
        };
      }
      else {
        delete $scope.alerts["deployments"];
      }
    }

    LabelFilter.onActiveFiltersChanged(function(labelSelector) {
      // trigger a digest loop
      $scope.$apply(function() {
        $scope.deployments = labelSelector.select($scope.unfilteredDeployments);
        updateFilterWarning();
      });
    }); 

    $scope.startLatestDeployment = function(deploymentConfig) {
      DeploymentsService.startLatestDeployment(deploymentConfig, $scope);
    };

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
