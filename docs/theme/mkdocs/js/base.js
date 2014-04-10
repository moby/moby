$(document).ready(function()
{
  prettyPrint();

  // Resizing
  resizeMenuDropdown();
  checkToScrollTOC();

  $(window).on('resize', function()
  {
    resizeMenuDropdown();
    checkToScrollTOC();
  });

  /* Auto scroll */
  $('#nav_menu').scrollToFixed({
    dontSetWidth: true,
  });

  /* Toggle TOC view for Mobile */
  $('#toc_table').on('click', function ()
  {
    if ( $(window).width() <= 991 )
    {
      $('#toc_table > #toc_navigation').slideToggle();
    }
  })

  /* Follow TOC links (ScrollSpy) */
  $('body').scrollspy({
    target: '#toc_table',
  });

  /* Prevent disabled link clicks */
  $("li.disabled a").click(function() {
    event.preventDefault();
  });

});

function resizeMenuDropdown ()
{
  if ( $(window).width() >= 768 )
  {
    $('#main-nav > li > .submenu').css("max-height", ($('body').height() - 160) + 'px');
  }
}

// https://github.com/bigspotteddog/ScrollToFixed
function checkToScrollTOC ()
{
  if ( $(window).width() > 999 )
  {
    if ( $('#toc_table').height() >= $(window).height() )
    {
      $('#toc_table').trigger('detach.ScrollToFixed');
    }
    else
    {
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
}
