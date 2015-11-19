'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServiceController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('ServiceController', function ($scope, $routeParams, DataService, project, $filter) {
    $scope.service = null;
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};    
    $scope.renderOptions.hideFilterWidget = true;    
    $scope.breadcrumbs = [
      {
        title: "Services",
        link: "project/" + $routeParams.project + "/browse/services"
      },
      {
        title: $routeParams.service
      }
    ];

    var watches = [];

    project.get($routeParams.project).then(function(resp) {
      angular.extend($scope, {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      });
      DataService.get("services", $routeParams.service, $scope).then(
        // success
        function(service) {
          $scope.loaded = true;
          $scope.service = service;

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("services", $routeParams.service, $scope, function(service, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This service has been deleted."
              }; 
            }
            $scope.service = service;
          }));          
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The service details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      watches.push(DataService.watch("routes", $scope, function(routes) {
        $scope.routesForService = [];
        angular.forEach(routes.by("metadata.name"), function(route) {
          if (route.spec.to.kind === "Service" &&
              route.spec.to.name === $routeParams.service) {
            $scope.routesForService.push(route);
          }
        });

        Logger.log("routes (subscribe)", $scope.routesByService);
      }));
    });

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
