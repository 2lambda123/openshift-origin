angular.module('openshiftConsole')
  .directive('podTemplate', function() {
    return {
      restrict: 'E',    
      templateUrl: 'views/_pod-template.html'
    };
  })
  .directive('pods', function() {
    return {
      restrict: 'E',
      scope: {
        pods: '='
      },
      templateUrl: 'views/_pods.html'
    };
  });