$(document).ready(function() {
    /* Prettyify */
    prettyPrint();

    $('#stickynav').scrollToFixed();

    $('#left_menu_navigation').scrollToFixed({
        marginTop: $('#stickynav').height(),
        limit: $('#footer').offset().top - $('#left_menu_navigation').height() - 450,
        zIndex: 1,
    });

    // /* Sticky TOC via Affix plug-in */
    // $('#left_menu_navigation').affix({
    //     offset: {
    //         top: $('#left_menu_navigation').offset().top,
    //         bottom: 450,
    //     }
    // });
    // $('#stickynav').affix({
    //     offset: {
    //         top: ($('#stickynav').offset().top) + 
    //              ($('#horizontal_thin_menu').offset().top) +
    //              ($('#topmostnav').offset().top),
    //     }
    // });

});

// $(window).on("resize", function() {
//     var isWindowTallEnough = $('#left_menu_navigation').height() + 20 < $(window).height();

//     console.log(isWindowTallEnough);

//     if (isWindowTallEnough) {
//         $('#left_menu_navigation').removeClass("affix affix-top");
//     } else {
//         $('#left_menu_navigation').addClass("affix affix-top");
//     }
// });

// /* Scrollspy */
// var navHeight = $('#left_menu_navigation').outerHeight(true) + 10;
// $('body').scrollspy({
//     target: '#toc_navigation',
//     offset: navHeight
// });


// /* Prevent disabled links from causing a page reload */
// $("li.disabled a").click(function() {
//     event.preventDefault();
// });


// /* Adjust the scroll height of anchors to compensate for the fixed navbar */
// window.disableShift = false;
// var shiftWindow = function() {
//     if (window.disableShift) {
//         window.disableShift = false;
//     } else {
//         /* If we're at the bottom of the page, don't erronously scroll up */
//         var scrolledToBottomOfPage = (
//             (window.innerHeight + window.scrollY) >= document.body.offsetHeight
//         );
//         if (!scrolledToBottomOfPage) {
//             scrollBy(0, -60);
//         };
//     };
// };
// if (location.hash) {shiftWindow();}
// window.addEventListener("hashchange", shiftWindow);


// /* Deal with clicks on nav links that do not change the current anchor link. */
// $("ul.nav a" ).click(function() {
//     var href = this.href;
//     var suffix = location.hash;
//     var matchesCurrentHash = (href.indexOf(suffix, href.length - suffix.length) !== -1);
//     if (location.hash && matchesCurrentHash) {
//         /* Force a single 'hashchange' event to occur after the click event */
//         window.disableShift = true;
//         location.hash='';
//     };
// });
