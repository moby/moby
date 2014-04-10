
 // $(document).bind("mobileinit", function(){
 //    $.extend($.mobile, {
 //      hashListeningEnabled: false,
 //      ajaxEnabled: false,
 //      pushStateEnabled: false,
 //    });
 //  });

$(document).ready(function() {

  // Prettyify
  prettyPrint();

  // Resizing
  resizeMenuDropdown();
  checkToScrollTOC();

  $('#nav_menu').scrollToFixed({
    dontSetWidth: true,
    // minWidth: 992,
  });

  console.log($('#toc_table').height() >= $(window).height());

  $(window).on('resize', function() {
    resizeMenuDropdown();
    checkToScrollTOC();
  });

  $('#toc_table').on('click', function () {
    $('#toc_table > #toc_navigation').toggle();
  })

});

function resizeMenuDropdown () {
  if ( $(window).width() >= 768 ) {
    $('#main-nav > li > .submenu').css("max-height", ($('body').height() - 160) + 'px');
  }
}

// https://github.com/bigspotteddog/ScrollToFixed
function checkToScrollTOC () {

  if ( $(window).width() > 768 ) {

    if ( $('#toc_table').height() >= $(window).height() ) {
      $('#toc_table').trigger('detach.ScrollToFixed');
    } else {
      $('#toc_table').scrollToFixed({
        marginTop: $('#nav_menu').height() + 14,
        limit: function () { return $('#footer').offset().top - 450; },
        // bottom: 0,
        zIndex: 1,
        minWidth: 769,
        removeOffsets: true,
      });
    }

  }

  // else

  // {

  //   $('#toc_table').scrollToFixed({
  //     marginTop: $('#nav_menu').height() + 14,
  //   });

  // }

}

// /* Scrollspy */
// var navHeight = $('#toc_table').outerHeight(true) + 10;
// $('body').scrollspy({
//   target: '#toc_navigation',
//   offset: navHeight
// });

// /* Prevent disabled links from causing a page reload */
// $("li.disabled a").click(function() {
//   event.preventDefault();
// });
