'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodController', function ($scope, $routeParams, DataService, project, $filter, ImageStreamResolver) {
    $scope.pod = null;
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};     
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};    
    $scope.renderOptions.hideFilterWidget = true;    
    $scope.breadcrumbs = [
      {
        title: "Pods",
        link: "project/" + $routeParams.project + "/browse/pods"
      },
      {
        title: $routeParams.pod
      }
    ];

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("pods", $routeParams.pod, $scope).then(
        // success
        function(pod) {
          $scope.pod = pod;
          var pods = {};
          pods[pod.metadata.name] = pod;
          ImageStreamResolver.fetchReferencedImageStreamImages(pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("pods", $routeParams.pod, $scope, function(pod, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This pod has been deleted."
              }; 
            }
            $scope.pod = pod;
          }));          
        },
        // failure
        function(e) {
          $scope.alerts["load"] = {
            type: "error",
            message: "The pod details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      // Sets up subscription for imageStreams
      watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
        $scope.imageStreams = imageStreams.by("metadata.name");
        ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
        ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
        Logger.log("imagestreams (subscribe)", $scope.imageStreams);
      }));

      watches.push(DataService.watch("builds", $scope, function(builds) {
        $scope.builds = builds.by("metadata.name");
        Logger.log("builds (subscribe)", $scope.builds);
      }));      
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });    
  });
