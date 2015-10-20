'use strict';

angular.module('openshiftConsole')
  // This triggers when an element has either a toggle or data-toggle attribute set on it
  .directive('toggle', function() {
    return {
      restrict: 'A',
      link: function($scope, element, attrs) {
        if (attrs) {
          switch(attrs.toggle) {
            case "popover":
              $(element).popover();
              break;
            case "tooltip":
              $(element).tooltip();
              break;
            case "dropdown":
              if (attrs.hover === "dropdown") {
                $(element).dropdownHover({delay: 200});
                $(element).dropdown();
              }
              break;
          }
        }
      }
    };
  })
  .directive('podWarnings', function(podWarningsFilter) {
    return {
      restrict:'E',
      scope: {
        pod: '='
      },
      link: function($scope, element) {
        var warnings = podWarningsFilter($scope.pod);
        var content = "";
        angular.forEach(warnings, function(warning) {
          content += warning.message + "<br>";
        });       
        $('.pficon-warning-triangle-o', element)
          .attr("data-content", content)
          .popover("destroy")
          .popover();
      },
      templateUrl: 'views/directives/_pod-warnings.html'
    };
  });
