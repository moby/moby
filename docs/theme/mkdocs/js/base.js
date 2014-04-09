$(document).ready(function() {
    
    // Prettyify
    prettyPrint();

    $('#stickynav').scrollToFixed({
        dontSetWidth: true,
        minWidth: 992,
    });

    // https://github.com/bigspotteddog/ScrollToFixed
    $('#left_menu_navigation').scrollToFixed({
        marginTop: $('#stickynav').height(),
        limit: function () {
            if ( $('#left_menu_navigation').height() + 120 >= $(window).height() ) {
                console.log('bla');
                return 1;
            }
            return $('#footer').offset().top - $('#footer').height() - 120;
        },
        zIndex: 1,
        minWidth: 768,
    });

    console.log($('#left_menu_navigation').height() >= $(window).height());

});


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
